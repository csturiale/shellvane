package app

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

func TestVaultAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket")
	}
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
	if err := v.Setup(ctx, []byte("password!")); err != nil {
		t.Fatal(err)
	}
	repo := keys.NewRepo(db, v)
	for _, kind := range []string{"ed25519", "rsa"} {
		priv, _ := keys.Generate(kind, 3072)
		if _, err := repo.Add(ctx, kind+"-key", priv, ""); err != nil {
			t.Fatal(err)
		}
	}
	sessions := NewSessionService(nil, nil, nil, nil, func(string, any) {})
	sessions.dataDir = dir
	svc := NewKeyService(repo, sessions)
	asked := 0
	allow := true
	svc.confirmSign = func(sshengine.SignRequest) sshengine.SignDecision {
		asked++
		if allow {
			return sshengine.SignOnce
		}
		return sshengine.SignRefuse
	}
	st, err := svc.StartAgent(ctx)
	if err != nil || !st.Running {
		t.Fatalf("StartAgent = %+v, %v", st, err)
	}
	defer svc.StopAgent()

	conn, err := net.Dial("unix", st.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := agent.NewClient(conn)
	list, err := client.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %v, %v", list, err)
	}
	for _, k := range list {
		pub, _ := ssh.ParsePublicKey(k.Blob)
		sig, err := client.SignWithFlags(pub, []byte("data"), agent.SignatureFlagRsaSha256)
		if err != nil {
			t.Fatalf("%s: %v", k.Comment, err)
		}
		if err := pub.Verify([]byte("data"), sig); err != nil {
			t.Fatalf("%s: bad signature: %v", k.Comment, err)
		}
	}
	allow = false
	pub, _ := ssh.ParsePublicKey(list[0].Blob)
	if _, err := client.Sign(pub, []byte("data")); err == nil {
		t.Fatal("refused signature succeeded")
	}
	if asked != 3 {
		t.Errorf("asked %d times", asked)
	}
	if err := client.RemoveAll(); err == nil {
		t.Error("RemoveAll accepted")
	}
	v.Lock()
	if list, _ := client.List(); len(list) != 0 {
		t.Errorf("locked vault still lists %d keys", len(list))
	}
}
