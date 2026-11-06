// Package otp computes time-based one-time passwords (RFC 6238), used to
// answer 2FA prompts with a seed kept in the vault.
package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrInvalidSeed rejects seeds that are not base32 or otpauth:// URIs.
const ErrInvalidSeed errs.Code = "otp.invalidSeed"

// Config is a TOTP generator.
type Config struct {
	Secret    []byte
	Digits    int
	Period    time.Duration
	Algorithm string // SHA1, SHA256 or SHA512
}

// Parse accepts a base32 secret (spaces and lower case allowed) or an
// otpauth://totp/... URI as shown by QR codes.
func Parse(seed string) (Config, error) {
	c := Config{Digits: 6, Period: 30 * time.Second, Algorithm: "SHA1"}
	seed = strings.TrimSpace(seed)
	if strings.HasPrefix(strings.ToLower(seed), "otpauth://") {
		u, err := url.Parse(seed)
		if err != nil || u.Host != "totp" {
			return c, errs.New(ErrInvalidSeed, nil)
		}
		q := u.Query()
		seed = q.Get("secret")
		if d, err := strconv.Atoi(q.Get("digits")); err == nil && (d == 6 || d == 7 || d == 8) {
			c.Digits = d
		}
		if p, err := strconv.Atoi(q.Get("period")); err == nil && p > 0 {
			c.Period = time.Duration(p) * time.Second
		}
		if a := strings.ToUpper(q.Get("algorithm")); a == "SHA256" || a == "SHA512" {
			c.Algorithm = a
		}
	}
	clean := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(seed, " ", ""), "-", ""))
	clean = strings.TrimRight(clean, "=")
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(clean)
	if err != nil || len(secret) < 10 {
		return c, errs.New(ErrInvalidSeed, nil)
	}
	c.Secret = secret
	return c, nil
}

// Code returns the code valid at t.
func (c Config) Code(t time.Time) string {
	counter := uint64(t.Unix()) / uint64(c.Period/time.Second)
	var newHash func() hash.Hash
	switch c.Algorithm {
	case "SHA256":
		newHash = sha256.New
	case "SHA512":
		newHash = sha512.New
	default:
		newHash = sha1.New
	}
	mac := hmac.New(newHash, c.Secret)
	binary.Write(mac, binary.BigEndian, counter)
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for range c.Digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", c.Digits, v%mod)
}

// LooksLikeOTPPrompt tells whether a keyboard-interactive question asks for
// a one-time code.
func LooksLikeOTPPrompt(question string) bool {
	q := strings.ToLower(question)
	for _, w := range []string{"verification code", "one-time", "one time", "otp", "token", "2fa", "authenticator", "totp", "codice", "passcode"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}
