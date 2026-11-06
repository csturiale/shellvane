package sshengine

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestProxyCommandRefusesShellCharacters(t *testing.T) {
	for _, tc := range []Target{{Host: "a;touch /tmp/x", Port: 22, User: "u"}, {Host: "web", Port: 22, User: "$(id)"}, {Host: "-oX", Port: 22}} {
		_, err := ProxyCommandDialer("nc %h %p # %r", tc)(context.Background(), "tcp", "")
		if errs.CodeOf(err) != ErrProxy {
			t.Errorf("%+v: %v", tc, err)
		}
	}
	if got := expandProxyCommand("ssh -W %h:%p %r@jump %%", Target{Host: "web", Port: 2222, User: "me"}); got != "ssh -W web:2222 me@jump %" {
		t.Errorf("expand = %q", got)
	}
	if !shellSafe("web.example.com") || !shellSafe("fe80::1%en0") || shellSafe("a b") {
		t.Error("shellSafe")
	}
}
