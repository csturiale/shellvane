package sshengine

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestConfirmingAgent(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	keyring := agent.NewKeyring()
	keyring.Add(agent.AddedKey{PrivateKey: priv, Comment: "me@laptop"})
	keys, _ := keyring.List()
	pub := keys[0]

	answers := []SignDecision{SignRefuse, SignOnce, SignSession}
	var asked []SignRequest
	c := NewConfirmingAgent(keyring.(agent.ExtendedAgent), func(r SignRequest) SignDecision {
		asked = append(asked, r)
		d := answers[0]
		answers = answers[1:]
		return d
	})

	if _, err := c.Sign(pub, []byte("a")); !errors.Is(err, ErrSignRefused) {
		t.Fatalf("refused: %v", err)
	}
	if _, err := c.Sign(pub, []byte("b")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Sign(pub, []byte("c")); err != nil {
		t.Fatal(err)
	}
	// Allowed for the session: no more questions.
	if _, err := c.Sign(pub, []byte("d")); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 3 || asked[0].Comment != "me@laptop" || asked[0].Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Fatalf("asked = %+v", asked)
	}
	if err := c.RemoveAll(); err == nil {
		t.Fatal("the remote side must not be able to remove keys")
	}
	if list, _ := c.List(); len(list) != 1 {
		t.Fatal("listing keys must still work")
	}
}
