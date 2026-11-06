package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDeployCommandIsIdempotentAndQuoted(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no POSIX shell")
	}
	home := t.TempDir()
	line := `ssh-ed25519 AAAAC3Nza me@it's; rm -rf /`
	for i := 0; i < 2; i++ {
		cmd := exec.Command("sh", "-c", deployCommand(line))
		cmd.Env = append(os.Environ(), "HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), line) != 1 || string(data) != line+"\n" {
		t.Fatalf("authorized_keys = %q", data)
	}
	// The command runs on the server, normally Unix; Windows has no mode bits.
	if fi, _ := os.Stat(filepath.Join(home, ".ssh")); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Errorf(".ssh mode = %v", fi.Mode().Perm())
	}
}
