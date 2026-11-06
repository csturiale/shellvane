package app

import (
	"context"
	"sync"
	"testing"

	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

// scripted answers prompts in order and records their kinds.
type scripted struct {
	mu      sync.Mutex
	svc     *SessionService
	answers []PromptAnswer
	kinds   []string
}

func (s *scripted) emit(name string, data any) {
	p, ok := data.(Prompt)
	if name != EventSessionPrompt || !ok {
		return
	}
	s.mu.Lock()
	s.kinds = append(s.kinds, p.Kind)
	a := PromptAnswer{Cancel: true}
	if len(s.answers) > 0 {
		a, s.answers = s.answers[0], s.answers[1:]
	}
	s.mu.Unlock()
	go s.svc.Answer(p.ID, a)
}

func setup(t *testing.T, answers ...PromptAnswer) (*prompter, *VaultService, *scripted) {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sc := &scripted{answers: answers}
	vs := NewVaultService(vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1}), func(string, any) {})
	svc := NewSessionService(nil, nil, vs, nil, sc.emit)
	sc.svc = svc
	return &prompter{svc: svc, sessionID: "s1", tried: map[string]bool{}}, vs, sc
}

var target = sshengine.Target{User: "deploy", Host: "10.0.0.1", Port: 22}

func TestPasswordIsSavedOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t, PromptAnswer{Value: "pw", Save: true})
	if err := vs.Setup(ctx, "master password"); err != nil {
		t.Fatal(err)
	}

	got, err := p.Password(ctx, target, 1)
	if err != nil || got != "pw" || sc.kinds[0] != "password" {
		t.Fatalf("Password = %q, %v, kinds %v", got, err, sc.kinds)
	}
	scope := vault.PasswordScope("deploy", "10.0.0.1", 22)
	if has, _ := vs.v.HasCredential(ctx, scope); has {
		t.Fatal("nothing may be saved before the login succeeds")
	}
	p.commit(ctx)
	if secret, found, _ := vs.v.Credential(ctx, scope); !found || string(secret) != "pw" {
		t.Fatalf("saved = %q, %v", secret, found)
	}
}

func TestSavedPasswordIsUsedOnceThenAsked(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t, PromptAnswer{Value: "typed"})
	vs.Setup(ctx, "master password")
	vs.v.SaveCredential(ctx, vault.PasswordScope("deploy", "10.0.0.1", 22), "x", []byte("saved"))

	if got, _ := p.Password(ctx, target, 1); got != "saved" || len(sc.kinds) != 0 {
		t.Fatalf("first attempt should use the vault silently: %q %v", got, sc.kinds)
	}
	// The saved password was wrong: the next attempt asks the user.
	if got, _ := p.Password(ctx, target, 2); got != "typed" {
		t.Fatalf("second attempt = %q", got)
	}
}

func TestLockedVaultIsUnlockedOnDemand(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t, PromptAnswer{Value: "wrong master"}, PromptAnswer{Value: "master password"})
	vs.Setup(ctx, "master password")
	vs.v.SaveCredential(ctx, vault.PasswordScope("deploy", "10.0.0.1", 22), "x", []byte("saved"))
	vs.Lock(ctx)

	got, err := p.Password(ctx, target, 1)
	if err != nil || got != "saved" {
		t.Fatalf("Password = %q, %v", got, err)
	}
	if len(sc.kinds) != 2 || sc.kinds[0] != "vaultUnlock" || sc.kinds[1] != "vaultUnlock" {
		t.Fatalf("prompts = %v, want two unlock attempts", sc.kinds)
	}
}

func TestCancelledUnlockFallsBackToPrompt(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t, PromptAnswer{Cancel: true}, PromptAnswer{Value: "typed"})
	vs.Setup(ctx, "master password")
	vs.v.SaveCredential(ctx, vault.PasswordScope("deploy", "10.0.0.1", 22), "x", []byte("saved"))
	vs.Lock(ctx)

	if got, _ := p.Password(ctx, target, 1); got != "typed" {
		t.Fatalf("Password = %q", got)
	}
	if sc.kinds[0] != "vaultUnlock" || sc.kinds[1] != "password" {
		t.Fatalf("prompts = %v", sc.kinds)
	}
}
