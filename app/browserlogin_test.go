package app

import (
	"context"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/sshengine"
)

func TestBrowserLoginWaitsForTheBookmarklet(t *testing.T) {
	opened := ""
	openInBrowser = func(link, browser string) error { opened = link + "|" + browser; return nil }
	defer func() { openInBrowser = func(string, string) error { return nil } }()

	if ReceiveSignInLink("shellvane://signin?s%3Atoken=abc") {
		t.Fatal("a link with no sign-in waiting must be dropped")
	}
	var got []map[string]string
	done := make(chan error, 1)
	go func() {
		done <- BrowserLoginLink(context.Background(), "CyberArk", "https://pvwa.example/PasswordVault/v10/logon/saml", "safari", func(v map[string]string) bool {
			got = append(got, v)
			return v["s:token"] == "good-token-0123456789"
		})
	}()
	send := func(link string) {
		for i := 0; i < 100 && !ReceiveSignInLink(link); i++ {
			time.Sleep(5 * time.Millisecond)
		}
	}
	send("shellvane://signin?s%3Atoken=early") // refused: keeps waiting
	send("shellvane://signin?s%3Atoken=good-token-0123456789")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if opened != "https://pvwa.example/PasswordVault/v10/logon/saml|safari" || len(got) != 2 {
		t.Fatalf("opened %q, got %v", opened, got)
	}
	if ReceiveSignInLink("shellvane://signin?s%3Atoken=late") {
		t.Fatal("the sign-in is over: later links are dropped")
	}
}

func TestBrowserLoginCancelled(t *testing.T) {
	openInBrowser = func(string, string) error { return nil }
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	err := BrowserLoginLink(ctx, "CyberArk", "https://pvwa.example", "", func(map[string]string) bool { return false })
	if errs.CodeOf(err) != sshengine.ErrCancelled {
		t.Fatalf("err = %v", err)
	}
	if IsSignInLink("shellvane://procedure?x=1") || !IsSignInLink("SHELLVANE://signin?a=b") {
		t.Fatal("IsSignInLink")
	}
}

func TestBrowserArgvKeepsTheLinkWhole(t *testing.T) {
	if browserArgv("no-such-browser", "darwin", "https://x.example") != nil {
		t.Fatal("unknown browser")
	}
}
