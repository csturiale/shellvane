package app

import (
	"reflect"
	"testing"
)

func TestSignInLinks(t *testing.T) {
	for link, want := range map[string]bool{
		"https://accounts.google.com/o/oauth2/auth?response_type=code&client_id=32555940559.apps": true,
		"https://login.microsoftonline.com/common/oauth2/v2.0/devicecode":                         true,
		"https://microsoft.com/devicelogin":                                                       true,
		"https://cyberark.example.com/PasswordVault/v10/logon/saml?SAMLRequest=abc":               true,
		"https://cloud.google.com/sdk/docs/install":                                               false,
		"http://accounts.google.com/o/oauth2/auth?client_id=x":                                    false,
		"https://localhost:8085/?code=abc":                                                        false,
	} {
		if got := isSignInLink(link); got != want {
			t.Errorf("isSignInLink(%q) = %v", link, got)
		}
	}

	var opened []string
	w := newLinkWatcher(func(l string) { opened = append(opened, l) })
	w.feed([]byte("Go to the following link in your browser:\r\n\r\n    \x1b[1mhttps://accounts.google.com/o/oauth2/auth?client_id=1&scope=openid"))
	if len(opened) != 0 {
		t.Fatal("opened before the line was complete")
	}
	w.feed([]byte("\x1b[0m\r\n\r\nEnter authorization code: "))
	w.feed([]byte("see https://cloud.google.com/sdk/docs for help.\nhttps://accounts.google.com/o/oauth2/auth?client_id=1&scope=openid\n"))
	if !reflect.DeepEqual(opened, []string{"https://accounts.google.com/o/oauth2/auth?client_id=1&scope=openid"}) {
		t.Fatalf("opened = %q", opened)
	}
	for i := 0; i < 10; i++ {
		w.feed([]byte("https://login.example.com/authorize?client_id=" + string(rune('a'+i)) + "\n"))
	}
	if len(opened) != maxOpenedLinks {
		t.Fatalf("opened %d links, want at most %d", len(opened), maxOpenedLinks)
	}
}

func TestSplitCommand(t *testing.T) {
	got := splitCommand(`open -na "Google Chrome" --args --incognito %s`)
	want := []string{"open", "-na", "Google Chrome", "--args", "--incognito", "%s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split = %q", got)
	}
	if got := splitCommand(`"C:/Program Files/x/msedge.exe" --inprivate %s`); got[0] != "C:/Program Files/x/msedge.exe" || len(got) != 3 {
		t.Fatalf("split = %q", got)
	}
}
