//go:build windows

package localshell

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Programs lists the shells installed on this computer, PowerShell first.
func Programs() []Program {
	var out []Program
	look := func(id, name, file string, args ...string) {
		if path, err := exec.LookPath(file); err == nil {
			out = append(out, Program{ID: id, Name: name, Path: path, Args: args})
		}
	}
	look("pwsh", "PowerShell", "pwsh.exe", "-NoLogo")
	look("powershell", "Windows PowerShell", "powershell.exe", "-NoLogo")
	look("cmd", "Command Prompt", "cmd.exe")
	look("wsl", "WSL", "wsl.exe")
	for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		bash := filepath.Join(dir, "Git", "bin", "bash.exe")
		if dir != "" {
			if _, err := os.Stat(bash); err == nil {
				out = append(out, Program{ID: "gitbash", Name: "Git Bash", Path: bash, Args: []string{"--login", "-i"}})
				break
			}
		}
	}
	return out
}
