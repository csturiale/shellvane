package sshengine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/knownhosts"
)

// TestKerberosLogin needs a KDC for SHELLVANE.TEST, an sshd accepting only
// gssapi-with-mic on 127.0.0.1:2223 (host/localhost in its keytab), the
// user shellvane (password shellvane) and a kinit ticket in /tmp/krb5cc_<uid>:
// set SHELLVANE_KERBEROS_TEST=1.
func TestKerberosLogin(t *testing.T) {
	if os.Getenv("SHELLVANE_KERBEROS_TEST") == "" {
		t.Skip("set SHELLVANE_KERBEROS_TEST=1 with a test KDC and sshd")
	}
	t.Setenv("SSH_AUTH_SOCK", "")
	target := Target{Host: "localhost", Port: 2223, User: "shellvane"}
	dial := func(p *fakePrompter) error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		opts := options(t, p)
		opts.Kerberos = &Kerberos{}
		c, err := Dial(ctx, target, opts)
		if err == nil {
			c.Close()
		}
		return err
	}
	// With the ticket of kinit (the credentials cache): no question.
	p := &fakePrompter{decision: knownhosts.AcceptOnce}
	if err := dial(p); err != nil || len(p.challenges) != 0 {
		t.Fatalf("with a ticket: %v (asked %d)", err, len(p.challenges))
	}
	// Without a ticket: the password gets one.
	t.Setenv("KRB5CCNAME", "FILE:"+t.TempDir()+"/none")
	p = &fakePrompter{decision: knownhosts.AcceptOnce, answers: [][]string{{"shellvane"}}}
	if err := dial(p); err != nil || len(p.challenges) != 1 {
		t.Fatalf("with the password: %v (asked %d)", err, len(p.challenges))
	}
	// A wrong password is refused.
	p = &fakePrompter{decision: knownhosts.AcceptOnce, answers: [][]string{{"nope"}}}
	if err := dial(p); err == nil {
		t.Fatal("wrong password accepted")
	}
}
