package pkcs11

import "github.com/csturiale/shellvane/core/errs"

// Error codes.
const (
	ErrUnsupported errs.Code = "pkcs11.unsupported"
	ErrModule      errs.Code = "pkcs11.module"
	ErrNoToken     errs.Code = "pkcs11.noToken"
	ErrNoKey       errs.Code = "pkcs11.noKey"
	ErrWrongPIN    errs.Code = "pkcs11.wrongPin"
	ErrPINLocked   errs.Code = "pkcs11.pinLocked"
	ErrSign        errs.Code = "pkcs11.sign"
)

// Token is a hardware token (or a soft one) seen by a module.
type Token struct {
	Label        string `json:"label"`
	Serial       string `json:"serial"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	NeedsPIN     bool   `json:"needsPin"`
}

// Key is a key pair on a token, identified by its CKA_ID (hex).
type Key struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}

// CommonModules are the usual places of the PKCS#11 modules of OpenSC,
// YubiKey (ykcs11) and others, by operating system.
var CommonModules = map[string][]string{
	"linux": {
		"/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so", "/usr/lib/opensc-pkcs11.so", "/usr/lib64/opensc-pkcs11.so",
		"/usr/lib/x86_64-linux-gnu/libykcs11.so", "/usr/lib/libykcs11.so", "/usr/lib64/libykcs11.so",
		"/usr/lib/aarch64-linux-gnu/opensc-pkcs11.so", "/usr/lib/softhsm/libsofthsm2.so",
	},
	"darwin": {
		"/Library/OpenSC/lib/opensc-pkcs11.so", "/usr/local/lib/opensc-pkcs11.so", "/opt/homebrew/lib/opensc-pkcs11.so",
		"/usr/local/lib/libykcs11.dylib", "/opt/homebrew/lib/libykcs11.dylib",
	},
	"windows": {
		`C:\Windows\System32\opensc-pkcs11.dll`, `C:\Program Files\OpenSC Project\OpenSC\pkcs11\opensc-pkcs11.dll`,
		`C:\Program Files\Yubico\Yubico PIV Tool\bin\libykcs11.dll`,
	},
}
