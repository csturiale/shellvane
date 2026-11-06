package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

func fakeExec(out string, code int, calls *[]string) plugins.Exec {
	return func(_ context.Context, command string) (string, int, error) {
		*calls = append(*calls, command)
		return out, code, nil
	}
}

func TestDockerPanelAndActions(t *testing.T) {
	var calls []string
	exec := fakeExec(`{"Names":"db","Image":"postgres:16","State":"exited","Status":"Exited (0) 2 hours ago","Ports":""}
{"Names":"web","Image":"nginx","State":"running","Status":"Up 3 days","Ports":"0.0.0.0:80->80/tcp"}`, 0, &calls)
	p, err := Docker{}.Panel(context.Background(), nil, plugins.Target{}, exec)
	if err != nil {
		t.Fatal(err)
	}
	rows := p.Tables[0].Rows
	if len(rows) != 2 || rows[0][0] != "web" || rows[1][2] != "exited" {
		t.Fatalf("rows = %v", rows)
	}
	res, err := Docker{}.Run(context.Background(), nil, plugins.Target{}, "logs", "web", exec)
	if err != nil || res.Command != "docker logs -f --tail 200 web" {
		t.Fatalf("logs = %+v, %v", res, err)
	}
	if _, err := (Docker{}).Run(context.Background(), nil, plugins.Target{}, "restart", "web", exec); err != nil || calls[len(calls)-1] != "docker restart web" {
		t.Fatalf("restart: %v %v", err, calls)
	}
	if _, err := (Docker{}).Run(context.Background(), nil, plugins.Target{}, "stop", "web; rm -rf /", exec); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("an unsafe name was accepted: %v", err)
	}
	_, err = Docker{}.Panel(context.Background(), nil, plugins.Target{}, fakeExec("sh: docker: not found", 127, &calls))
	if errs.CodeOf(err) != plugins.ErrCommand || !strings.Contains(errs.ToPayload(err).Params["detail"].(string), "not found") {
		t.Fatalf("missing docker: %v", err)
	}
}

func TestSystemdAndLogs(t *testing.T) {
	var calls []string
	exec := fakeExec(`nginx.service loaded active running A high performance web server
backup.service loaded failed failed Nightly backup
cron.service loaded active running Regular background program processing daemon
tmp.service loaded inactive dead Cleanup`, 0, &calls)
	p, err := Systemd{}.Panel(context.Background(), nil, plugins.Target{}, exec)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tables[0].Rows) != 1 || p.Tables[0].Rows[0][0] != "backup.service" || len(p.Tables[1].Rows) != 2 || p.Tables[1].Rows[0][3] != "A high performance web server" {
		t.Fatalf("tables = %+v", p.Tables)
	}
	res, _ := Systemd{}.Run(context.Background(), nil, plugins.Target{}, "restart", "nginx.service", nil)
	if !strings.HasPrefix(res.Command, "sudo systemctl restart nginx.service") {
		t.Fatalf("restart = %+v", res)
	}
	if _, err := (Systemd{}).Run(context.Background(), nil, plugins.Target{}, "status", "$(reboot)", nil); err == nil {
		t.Fatal("an unsafe unit was accepted")
	}
	lp, err := Logs{}.Panel(context.Background(), nil, plugins.Target{}, fakeExec("Nov 03 kernel: error", 0, &calls))
	if err != nil || lp.Text != "Nov 03 kernel: error" {
		t.Fatalf("logs = %+v, %v", lp, err)
	}
}
