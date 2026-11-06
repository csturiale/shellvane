package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

const hostKeyMismatch Code = "ssh.hostKeyMismatch"

func TestErrorMessageIsDeterministic(t *testing.T) {
	err := Wrap(errors.New("boom"), hostKeyMismatch, Params{"port": 22, "host": "example.org"})
	want := "ssh.hostKeyMismatch (host=example.org, port=22): boom"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestCodeOfWrappedError(t *testing.T) {
	err := fmt.Errorf("connect: %w", New(hostKeyMismatch, nil))
	if got := CodeOf(err); got != hostKeyMismatch {
		t.Fatalf("CodeOf = %q, want %q", got, hostKeyMismatch)
	}
	if got := CodeOf(errors.New("plain")); got != Internal {
		t.Fatalf("CodeOf(plain) = %q, want %q", got, Internal)
	}
}

func TestIsMatchesByCode(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", Wrap(errors.New("x"), NotFound, Params{"id": 1}))
	if !errors.Is(err, New(NotFound, nil)) {
		t.Fatal("errors.Is should match on code")
	}
	if errors.Is(err, New(InvalidInput, nil)) {
		t.Fatal("errors.Is should not match a different code")
	}
}

func TestMarshal(t *testing.T) {
	var p Payload
	if err := json.Unmarshal(Marshal(New(NotFound, Params{"id": "h1"})), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != NotFound || p.Params["id"] != "h1" || p.Detail != "" {
		t.Fatalf("unexpected payload: %+v", p)
	}

	if err := json.Unmarshal(Marshal(errors.New("disk full")), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != Internal || p.Detail != "disk full" {
		t.Fatalf("unexpected payload for plain error: %+v", p)
	}
}

func TestErrorMarshalsAsPayload(t *testing.T) {
	var err error = Wrap(errors.New("cause"), NotFound, Params{"id": "h1"})
	b, jsonErr := json.Marshal(&err)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if want := `{"code":"notFound","params":{"id":"h1"},"detail":"cause"}`; string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}
