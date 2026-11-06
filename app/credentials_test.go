package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"

	"github.com/csturiale/shellvane/core/otp"
	"github.com/csturiale/shellvane/core/sshengine"
)

const totpSeed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // RFC 6238 test secret

func TestChallengeAutofillsTOTPAndAsksTheRest(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t, PromptAnswer{Answers: []string{"ignored", "typed"}})
	vs.Setup(ctx, "master password")
	p.host = resolvedHost{id: "h1", target: target}
	if err := vs.SetTOTP(ctx, "h1", totpSeed); err != nil {
		t.Fatal(err)
	}

	got, err := p.Challenge(ctx, target, sshengine.Challenge{Questions: []string{"Verification code: ", "Favourite colour: "}, Echo: []bool{false, true}, Round: 1})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := otp.Parse(totpSeed)
	// The code may have rolled over to the next 30 s step meanwhile.
	if (got[0] != cfg.Code(time.Now()) && got[0] != cfg.Code(time.Now().Add(-30*time.Second))) || got[1] != "typed" {
		t.Fatalf("answers = %v", got)
	}
	if len(sc.kinds) != 1 || sc.kinds[0] != "challenge" {
		t.Fatalf("prompts = %v", sc.kinds)
	}
}

func TestChallengeFullyAutofilledAsksNothing(t *testing.T) {
	ctx := context.Background()
	p, vs, sc := setup(t)
	vs.Setup(ctx, "master password")
	p.host = resolvedHost{id: "h1", target: target}
	vs.SetTOTP(ctx, "h1", totpSeed)

	got, err := p.Challenge(ctx, target, sshengine.Challenge{Questions: []string{"OTP: "}, Echo: []bool{false}, Round: 1})
	if err != nil || len(got) != 1 || len(got[0]) != 6 || len(sc.kinds) != 0 {
		t.Fatalf("answers = %v, %v, prompts %v", got, err, sc.kinds)
	}
}

func TestSetTOTPRejectsInvalidSeed(t *testing.T) {
	ctx := context.Background()
	_, vs, _ := setup(t)
	vs.Setup(ctx, "master password")
	if err := vs.SetTOTP(ctx, "h1", "not base32!"); err == nil {
		t.Fatal("invalid seed accepted")
	}
	if has, _ := vs.HasTOTP(ctx, "h1"); has {
		t.Fatal("seed stored")
	}
}

func TestPasswordCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	ctx := context.Background()
	p, _, sc := setup(t)
	p.host = resolvedHost{target: target, passwordCommand: `printf 'pw-%s-%s' "$SHELLVANE_USER" "$SHELLVANE_HOST"`}

	if got, err := p.Password(ctx, target, 1); err != nil || got != "pw-deploy-10.0.0.1" || len(sc.kinds) != 0 {
		t.Fatalf("Password = %q, %v, prompts %v", got, err, sc.kinds)
	}
	// A rejected command password is not retried: the user is asked.
	if _, err := p.Password(ctx, target, 2); err == nil || sc.kinds[0] != "password" {
		t.Fatalf("second attempt: %v, prompts %v", err, sc.kinds)
	}
}

func TestCredentialCommandFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	if _, err := runCredentialCommand(context.Background(), "exit 3", nil); err == nil {
		t.Fatal("failure not reported")
	}
	out, err := runCredentialCommand(context.Background(), `cat "$SHELLVANE_PUBLIC_KEY_FILE"`, map[string]string{"SHELLVANE_PUBLIC_KEY": "ssh-ed25519 AAAA"})
	if err != nil || out != "ssh-ed25519 AAAA" {
		t.Fatalf("out = %q, %v", out, err)
	}
}

func TestPreConnectCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	out := filepath.Join(t.TempDir(), "pushed")
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	ht := resolvedHost{name: "web", target: sshengine.Target{Host: "i-0abc", Port: 22, User: "ec2-user"},
		preConnectCommand: `echo "$SHELLVANE_HOST $SHELLVANE_USER $SHELLVANE_HOST_NAME $(cat "$SHELLVANE_PUBLIC_KEY_FILE")" > ` + out}
	if err := preConnect(context.Background(), ht, signer); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	want := "i-0abc ec2-user web " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("command saw %q, want %q", data, want)
	}

	ht.preConnectCommand = `echo "An error occurred (AccessDenied)" >&2; exit 254`
	err := preConnect(context.Background(), ht, nil)
	if errs.CodeOf(err) != ErrPreConnect || errs.ToPayload(err).Params["detail"] != "An error occurred (AccessDenied)" {
		t.Fatalf("err = %v (%+v)", err, errs.ToPayload(err).Params)
	}
}
