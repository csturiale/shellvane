package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/biometric"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/keychain"
	"github.com/csturiale/shellvane/core/otp"
	"github.com/csturiale/shellvane/core/vault"
)

// EventVaultState is emitted when the vault is set up, locked or unlocked.
const EventVaultState = "vault:state"

// VaultStatus is the payload of EventVaultState.
type VaultStatus struct {
	State vault.State `json:"state"`
}

// defaultAutoLock locks the vault after this much inactivity.
const defaultAutoLock = 15 * time.Minute

// VaultService exposes the vault to the frontend. Secrets never leave Go:
// the UI can manage credentials but not read them.
type VaultService struct {
	v    *vault.Vault
	emit Emitter

	// bio confirms the user before the keychain unlock; reason gives the
	// text of its dialog, in the app's language.
	bio     biometric.Verifier
	reason  func() string
	secrets vault.SecretStore // the system keychain; tests replace it

	mu          sync.Mutex
	autoLock    time.Duration // 0 never locks for inactivity
	lockOnSleep bool
	timer       *time.Timer
}

// NewVaultService returns a service over v. It watches for the computer
// going to sleep, to lock the vault when asked to.
func NewVaultService(v *vault.Vault, emit Emitter) *VaultService {
	s := &VaultService{v: v, emit: emit, autoLock: defaultAutoLock, lockOnSleep: true}
	go s.watchSleep(15 * time.Second)
	return s
}

// SetAutoLock sets the inactivity delay in minutes (0 = never) and whether
// the vault locks when the computer sleeps.
func (s *VaultService) SetAutoLock(minutes int, onSleep bool) {
	s.mu.Lock()
	s.autoLock = time.Duration(max(minutes, 0)) * time.Minute
	s.lockOnSleep = onSleep
	s.mu.Unlock()
	if st, _ := s.v.State(context.Background()); st == vault.StateUnlocked {
		s.touch()
	}
}

// watchSleep notices suspends: the wall clock moves on while the
// monotonic clock (which stops during sleep on most systems) does not.
func (s *VaultService) watchSleep(every time.Duration) {
	prev := time.Now()
	for range time.Tick(every) {
		now := time.Now()
		s.mu.Lock()
		on := s.lockOnSleep
		s.mu.Unlock()
		if on && slept(prev, now, every) {
			if st, _ := s.v.State(context.Background()); st == vault.StateUnlocked {
				s.Lock(context.Background())
			}
		}
		prev = now
	}
}

// slept reports a suspend between two readings taken about every apart:
// the wall clock (Round(0) drops the monotonic reading) advanced much more
// than expected.
func slept(prev, now time.Time, every time.Duration) bool {
	wall := now.Round(0).Sub(prev.Round(0))
	mono := now.Sub(prev)
	return wall-mono > 30*time.Second || mono > every+2*time.Minute
}

// CreateRecoveryKey makes a recovery key, shown once (the vault must be
// unlocked); it replaces any previous one.
func (s *VaultService) CreateRecoveryKey(ctx context.Context) (string, error) {
	return s.v.CreateRecoveryKey(ctx)
}

// HasRecoveryKey reports whether a recovery key exists.
func (s *VaultService) HasRecoveryKey(ctx context.Context) (bool, error) {
	return s.v.HasRecoveryKey(ctx)
}

// RemoveRecoveryKey forgets the recovery key.
func (s *VaultService) RemoveRecoveryKey(ctx context.Context) error {
	return s.v.RemoveRecoveryKey(ctx)
}

// Recover sets a new master password with the recovery key and unlocks.
func (s *VaultService) Recover(ctx context.Context, recoveryKey, newPassword string) error {
	if err := s.v.Recover(ctx, recoveryKey, []byte(newPassword)); err != nil {
		return err
	}
	s.touch()
	s.notify(ctx)
	return nil
}

// EnableKeychainUnlock lets the vault open with a secret kept in the
// system keychain instead of the master password. The vault must be
// unlocked.
func (s *VaultService) EnableKeychainUnlock(ctx context.Context) error {
	return s.v.EnableDeviceUnlock(ctx, s.keychain())
}

// DisableKeychainUnlock removes the keychain secret.
func (s *VaultService) DisableKeychainUnlock(ctx context.Context) error {
	s.v.SetDeviceBiometric(ctx, false) // it only guarded the keychain unlock
	return s.v.DisableDeviceUnlock(ctx, s.keychain())
}

// HasKeychainUnlock reports whether the keychain unlock is set up.
func (s *VaultService) HasKeychainUnlock(ctx context.Context) (bool, error) {
	return s.v.HasDeviceUnlock(ctx)
}

// UnlockWithKeychain opens the vault with the keychain secret; the system
// may ask the user to allow it, and Touch ID or Windows Hello confirms
// the user first when required.
func (s *VaultService) UnlockWithKeychain(ctx context.Context) error {
	if on, _ := s.v.DeviceBiometric(ctx); on {
		if err := s.verifier().Verify(ctx, s.reasonText()); err != nil {
			return err
		}
	}
	if err := s.v.UnlockWithDevice(ctx, s.keychain()); err != nil {
		return err
	}
	s.touch()
	s.notify(ctx)
	return nil
}

// EnableBiometric sets the verifier and the text of its dialog. It is not
// a method so that Wails does not expose it as a binding.
func EnableBiometric(s *VaultService, v biometric.Verifier, reason func() string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bio, s.reason = v, reason
}

func (s *VaultService) keychain() vault.SecretStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secrets == nil {
		return keychain.System{}
	}
	return s.secrets
}

func (s *VaultService) verifier() biometric.Verifier {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bio == nil {
		return biometric.System()
	}
	return s.bio
}

func (s *VaultService) reasonText() string {
	s.mu.Lock()
	r := s.reason
	s.mu.Unlock()
	if r == nil {
		return "Shellvane"
	}
	return r()
}

// BiometricStatus tells whether Touch ID or Windows Hello can confirm the
// keychain unlock, and whether it is required.
type BiometricStatus struct {
	Kind      string `json:"kind"` // "touchid", "hello", "" (none)
	Available bool   `json:"available"`
	Required  bool   `json:"required"`
}

// Biometric reports the verifier of this computer.
func (s *VaultService) Biometric(ctx context.Context) BiometricStatus {
	v := s.verifier()
	st := BiometricStatus{Kind: v.Kind()}
	if st.Kind != biometric.KindNone {
		st.Available = v.Available(ctx)
	}
	st.Required, _ = s.v.DeviceBiometric(ctx)
	return st
}

// SetBiometric requires (or stops requiring) Touch ID or Windows Hello
// before the keychain unlock. Turning it on asks once, to check it works.
func (s *VaultService) SetBiometric(ctx context.Context, on bool) error {
	if on {
		if has, _ := s.v.HasDeviceUnlock(ctx); !has {
			return errs.New(vault.ErrNoDeviceUnlock, nil)
		}
		if err := s.verifier().Verify(ctx, s.reasonText()); err != nil {
			return err
		}
	}
	return s.v.SetDeviceBiometric(ctx, on)
}

// Status returns the vault state.
func (s *VaultService) Status(ctx context.Context) (VaultStatus, error) {
	st, err := s.v.State(ctx)
	return VaultStatus{State: st}, err
}

// Setup creates the vault with a master password.
func (s *VaultService) Setup(ctx context.Context, password string) error {
	if err := s.v.Setup(ctx, []byte(password)); err != nil {
		return err
	}
	s.touch()
	s.notify(ctx)
	return nil
}

// Unlock opens the vault.
func (s *VaultService) Unlock(ctx context.Context, password string) error {
	if err := s.v.Unlock(ctx, []byte(password)); err != nil {
		return err
	}
	s.touch()
	s.notify(ctx)
	return nil
}

// Lock closes the vault.
func (s *VaultService) Lock(ctx context.Context) {
	s.v.Lock()
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.mu.Unlock()
	s.notify(ctx)
}

// ChangePassword replaces the master password.
func (s *VaultService) ChangePassword(ctx context.Context, oldPassword, newPassword string) error {
	return s.v.ChangePassword(ctx, []byte(oldPassword), []byte(newPassword))
}

// Credentials lists saved credentials (metadata only).
func (s *VaultService) Credentials(ctx context.Context) ([]vault.Credential, error) {
	return s.v.Credentials(ctx)
}

// DeleteCredential forgets a saved credential.
func (s *VaultService) DeleteCredential(ctx context.Context, scope string) error {
	return s.v.DeleteCredential(ctx, scope)
}

// SetTOTP stores a host's TOTP seed (base32 or otpauth:// URI) so 2FA
// prompts are answered automatically; an empty seed removes it.
func (s *VaultService) SetTOTP(ctx context.Context, hostID, seed string) error {
	if strings.TrimSpace(seed) == "" {
		return s.v.DeleteCredential(ctx, TOTPScope(hostID))
	}
	if _, err := otp.Parse(seed); err != nil {
		return err
	}
	return s.v.SaveCredential(ctx, TOTPScope(hostID), "TOTP", []byte(strings.TrimSpace(seed)))
}

// HasTOTP reports whether a host has a TOTP seed.
func (s *VaultService) HasTOTP(ctx context.Context, hostID string) (bool, error) {
	return s.v.HasCredential(ctx, TOTPScope(hostID))
}

// touch restarts the inactivity timer.
func (s *VaultService) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.autoLock > 0 {
		s.timer = time.AfterFunc(s.autoLock, func() { s.Lock(context.Background()) })
	}
}

func (s *VaultService) notify(ctx context.Context) {
	if st, err := s.v.State(ctx); err == nil {
		s.emit(EventVaultState, VaultStatus{State: st})
	}
}
