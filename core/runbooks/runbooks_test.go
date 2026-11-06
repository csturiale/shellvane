package runbooks

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

// fakeExec answers commands from a table; "fail" exits 1, "sleep" blocks.
type fakeExec struct {
	mu   sync.Mutex
	ran  []string
	busy int
	max  int
}

func (f *fakeExec) Exec(ctx context.Context, h Host, cmd string) Result {
	f.mu.Lock()
	f.ran = append(f.ran, h.Name+":"+cmd)
	f.busy++
	if f.busy > f.max {
		f.max = f.busy
	}
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.busy--; f.mu.Unlock() }()
	switch {
	case strings.HasPrefix(cmd, "fail"):
		return Result{ExitCode: 1, Output: "boom"}
	case strings.HasPrefix(cmd, "sleep"):
		select {
		case <-ctx.Done():
			return Result{ExitCode: -1}
		case <-time.After(50 * time.Millisecond):
		}
	}
	return Result{Output: "out-of-" + cmd + "\n"}
}

func wait(t *testing.T, r *Run) {
	t.Helper()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("run did not finish")
	}
}

func states(cells []Cell) string {
	var s []string
	for _, c := range cells {
		s = append(s, c.State)
	}
	return strings.Join(s, ",")
}

func TestStepsConditionsAndOutputs(t *testing.T) {
	rb := Runbook{Name: "deploy", Steps: []Step{
		{Name: "version", Command: "cat VERSION on {{host}} for {{env}}"},
		{Command: "echo {{steps.version.output}}"},
		{Command: "fail now", ContinueOnError: true},
		{Command: "rollback", When: WhenFailure},
		{Command: "notify", When: WhenSuccess},
		{Command: "check", When: WhenMatches, Pattern: "out-of-notify"},
		{Command: "never", When: WhenMatches, Pattern: "nothing like this"},
	}}
	if err := rb.Validate(); err != nil {
		t.Fatal(err)
	}
	x := &fakeExec{}
	run := Start(context.Background(), "r1", rb, []Host{{ID: "h1", Name: "web-1"}}, map[string]string{"env": "prod"}, x, nil)
	wait(t, run)
	got := run.Cells()["h1"]
	// Conditions look at the last step that ran: the rollback succeeded.
	if states(got) != "done,done,failed,done,done,done,skipped" {
		t.Fatalf("states %s", states(got))
	}
	if got[1].Command != "echo out-of-cat VERSION on web-1 for prod" {
		t.Fatalf("captured output: %q", got[1].Command)
	}
	plan := Plan(rb, []Host{{ID: "h1", Name: "web-1"}}, map[string]string{"env": "prod"})
	if plan["h1"][0] != "cat VERSION on web-1 for prod" || plan["h1"][1] != "echo {{steps.version.output}}" {
		t.Fatalf("plan %v", plan["h1"])
	}
}

func TestFailureStopsTheHostAndSequence(t *testing.T) {
	rb := Runbook{Name: "x", StopOnHostFailure: true, Steps: []Step{{Command: "fail"}, {Command: "after"}}}
	hosts := []Host{{ID: "a", Name: "a"}, {ID: "b", Name: "b"}}
	run := Start(context.Background(), "r", rb, hosts, nil, &fakeExec{}, nil)
	wait(t, run)
	cells := run.Cells()
	if states(cells["a"]) != "failed,cancelled" || states(cells["b"]) != "cancelled,cancelled" {
		t.Fatalf("a %s b %s", states(cells["a"]), states(cells["b"]))
	}
}

func TestParallelLimitAndCancel(t *testing.T) {
	rb := Runbook{Name: "x", Parallel: true, Concurrency: 2, Steps: []Step{{Command: "sleep"}}}
	var hosts []Host
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		hosts = append(hosts, Host{ID: n, Name: n})
	}
	x := &fakeExec{}
	run := Start(context.Background(), "r", rb, hosts, nil, x, nil)
	wait(t, run)
	if x.max != 2 || len(x.ran) != 5 {
		t.Fatalf("max %d ran %d", x.max, len(x.ran))
	}

	rb.Steps = []Step{{Command: "sleep"}, {Command: "after"}}
	run = Start(context.Background(), "r2", rb, hosts, nil, &fakeExec{}, nil)
	run.Cancel()
	wait(t, run)
	for _, h := range hosts {
		if c := run.Cells()[h.ID][1]; c.State != StateCancelled {
			t.Fatalf("%s: %s", h.ID, c.State)
		}
	}
}

func TestConfirm(t *testing.T) {
	rb := Runbook{Name: "x", Steps: []Step{{Command: "a"}, {Command: "b", Confirm: true}, {Command: "c", Confirm: true}}}
	waiting := make(chan int, 4)
	run := Start(context.Background(), "r", rb, []Host{{ID: "h", Name: "h"}}, nil, &fakeExec{}, func(_ string, step int, c Cell) {
		if c.State == StateWaiting {
			waiting <- step
		}
	})
	if <-waiting != 1 || !run.Confirm("", 1, true) {
		t.Fatal("first confirmation")
	}
	if <-waiting != 2 || !run.Confirm("h", 2, false) {
		t.Fatal("second confirmation")
	}
	wait(t, run)
	if states(run.Cells()["h"]) != "done,done,cancelled" {
		t.Fatalf("states %s", states(run.Cells()["h"]))
	}
}

func TestRepoYAMLAndExport(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepo(db)
	if _, err := repo.Save(ctx, Runbook{Name: "x"}); errs.CodeOf(err) != ErrNoSteps {
		t.Fatalf("no steps: %v", err)
	}
	if _, err := repo.Save(ctx, Runbook{Name: "x", Steps: []Step{{Command: "a", When: WhenMatches, Pattern: "("}}}); errs.CodeOf(err) != ErrBadCondition {
		t.Fatalf("pattern: %v", err)
	}
	rb, err := repo.Save(ctx, Runbook{Name: " Patch ", Steps: []Step{{Name: "up", Command: "apt-get -y upgrade"}},
		Targets: Targets{HostIDs: []string{"h1"}, GroupIDs: []string{"g1"}, Tags: []string{"prod"}}})
	if err != nil || rb.Name != "Patch" {
		t.Fatalf("save %+v %v", rb, err)
	}
	if list, _ := repo.List(ctx); len(list) != 1 {
		t.Fatal("list")
	}
	text, err := ExportYAML(rb, func(kind, id string) string { return kind + "-" + id })
	if err != nil || !strings.Contains(text, "- host-h1") || !strings.Contains(text, "- group-g1") {
		t.Fatalf("yaml %s", text)
	}
	in, err := ImportYAML(text, func(kind, name string) string { return strings.TrimPrefix(name, kind+"-") + "x" })
	if err != nil || in.Targets.HostIDs[0] != "h1x" || in.Targets.GroupIDs[0] != "g1x" || in.Targets.Tags[0] != "prod" {
		t.Fatalf("import %+v %v", in, err)
	}
	cells := map[string][]Cell{"h1": {{State: StateDone, Command: "apt-get -y upgrade", Result: &Result{Output: "ok, \"done\""}}}}
	csv, _ := ExportResults(rb, []Host{{ID: "h1", Name: "web-1"}}, cells, "csv")
	if !strings.Contains(csv, `web-1,up,apt-get -y upgrade,done,0,0.000,"ok, ""done"""`) {
		t.Fatalf("csv %s", csv)
	}
}
