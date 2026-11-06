package otp

import (
	"encoding/base32"
	"testing"
	"time"
)

// RFC 6238 appendix B: seeds are the ASCII strings below.
func TestRFC6238Vectors(t *testing.T) {
	seeds := map[string]string{
		"SHA1":   "12345678901234567890",
		"SHA256": "12345678901234567890123456789012",
		"SHA512": "1234567890123456789012345678901234567890123456789012345678901234",
	}
	cases := []struct {
		unix int64
		alg  string
		code string
	}{
		{59, "SHA1", "94287082"},
		{59, "SHA256", "46119246"},
		{59, "SHA512", "90693936"},
		{1111111109, "SHA1", "07081804"},
		{1234567890, "SHA256", "91819424"},
		{20000000000, "SHA512", "47863826"},
	}
	for _, c := range cases {
		cfg := Config{Secret: []byte(seeds[c.alg]), Digits: 8, Period: 30 * time.Second, Algorithm: c.alg}
		if got := cfg.Code(time.Unix(c.unix, 0)); got != c.code {
			t.Errorf("%s @%d = %s, want %s", c.alg, c.unix, got, c.code)
		}
	}
}

func TestParse(t *testing.T) {
	secret := base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	c, err := Parse("  " + secret[:8] + " " + secret[8:] + "  ")
	if err != nil || c.Digits != 6 || c.Algorithm != "SHA1" {
		t.Fatalf("plain seed: %+v %v", c, err)
	}
	c, err = Parse("otpauth://totp/ACME:me?secret=" + secret + "&digits=8&algorithm=SHA256&period=60")
	if err != nil || c.Digits != 8 || c.Algorithm != "SHA256" || c.Period != time.Minute {
		t.Fatalf("uri: %+v %v", c, err)
	}
	if _, err := Parse("not base32!"); err == nil {
		t.Fatal("garbage must be rejected")
	}
}

func TestLooksLikeOTPPrompt(t *testing.T) {
	for _, q := range []string{"Verification code: ", "Enter your OTP:", "Duo passcode", "Codice di verifica"} {
		if !LooksLikeOTPPrompt(q) {
			t.Errorf("%q should be an OTP prompt", q)
		}
	}
	if LooksLikeOTPPrompt("Password: ") {
		t.Error("a password prompt is not an OTP prompt")
	}
}
