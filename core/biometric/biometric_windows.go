//go:build windows

package biometric

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Windows Hello is a Windows Runtime API (UserConsentVerifier); Windows
// PowerShell 5.1, present on every Windows 10 and 11, can call it without
// any native code in Shellvane.
const helloPrelude = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTask = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
  $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1'
})[0]
function Await($op, $type) {
  $task = $asTask.MakeGenericMethod($type).Invoke($null, @($op))
  $task.Wait(-1) | Out-Null
  $task.Result
}
[Windows.Security.Credentials.UI.UserConsentVerifier, Windows.Security.Credentials.UI, ContentType = WindowsRuntime] | Out-Null
`

const helloAvailable = helloPrelude + `
Await ([Windows.Security.Credentials.UI.UserConsentVerifier]::CheckAvailabilityAsync()) ([Windows.Security.Credentials.UI.UserConsentVerifierAvailability])
`

const helloVerify = helloPrelude + `
Await ([Windows.Security.Credentials.UI.UserConsentVerifier]::RequestVerificationAsync($env:SHELLVANE_REASON)) ([Windows.Security.Credentials.UI.UserConsentVerificationResult])
`

type system struct{}

func (system) Kind() string { return KindHello }

func run(ctx context.Context, script, reason string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "SHELLVANE_REASON="+reason)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func (system) Available(ctx context.Context) bool {
	out, err := run(ctx, helloAvailable, "", 20*time.Second)
	return err == nil && out == "Available"
}

func (system) Verify(ctx context.Context, reason string) error {
	out, err := run(ctx, helloVerify, reason, 2*time.Minute)
	switch {
	case err != nil:
		return errs.Wrap(err, ErrFailed, nil)
	case out == "Verified":
		return nil
	case out == "Canceled":
		return errs.New(ErrCancelled, nil)
	case out == "DeviceNotPresent" || out == "NotConfiguredForUser" || out == "DisabledByPolicy":
		return errs.New(ErrUnavailable, nil)
	}
	return errs.New(ErrFailed, errs.Params{"detail": out})
}
