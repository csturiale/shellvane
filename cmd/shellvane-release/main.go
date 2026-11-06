// Command shellvane-release prepares the signed update manifest of a
// release. It is a maintainer tool used by CI, not shipped to users.
//
//	shellvane-release keygen
//	shellvane-release manifest -version 1.2.3 -base-url URL -page URL platform=file[:kind]...
//	shellvane-release sign manifest.json   (key in $SHELLVANE_UPDATE_KEY)
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen()
	case "manifest":
		err = manifest(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: shellvane-release keygen | manifest -version V -base-url URL [-page URL] platform=file[:kind]... | sign FILE (manifest.json or plugins/index.json)")
	os.Exit(2)
}

func keygen() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Println("public  (core/update.PublicKey):", base64.StdEncoding.EncodeToString(pub))
	fmt.Println("private (secret SHELLVANE_UPDATE_KEY):", base64.StdEncoding.EncodeToString(priv))
	return nil
}

func manifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := fs.String("version", "", "release version")
	baseURL := fs.String("base-url", "", "URL the files are downloaded from")
	page := fs.String("page", "", "release page")
	out := fs.String("o", "manifest.json", "output file")
	fs.Parse(args)
	if *version == "" || *baseURL == "" || fs.NArg() == 0 {
		return fmt.Errorf("version, base-url and at least one platform=file are required")
	}
	m := update.Manifest{Version: strings.TrimPrefix(*version, "v"), Date: time.Now().UTC().Format(time.RFC3339), Page: *page, Assets: map[string]update.Asset{}}
	for _, a := range fs.Args() {
		platform, spec, ok := strings.Cut(a, "=")
		if !ok {
			return fmt.Errorf("expected platform=file[:kind], got %q", a)
		}
		file, kind, _ := strings.Cut(spec, ":")
		if kind == "" {
			kind = "binary"
		}
		sum, err := update.FileSHA256(file)
		if err != nil {
			return err
		}
		m.Assets[platform] = update.Asset{URL: strings.TrimSuffix(*baseURL, "/") + "/" + filepath.Base(file), SHA256: sum, Kind: kind}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*out, append(data, '\n'), 0o644)
}

func sign(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("sign takes the manifest file")
	}
	key := os.Getenv("SHELLVANE_UPDATE_KEY")
	if key == "" {
		return fmt.Errorf("SHELLVANE_UPDATE_KEY is not set")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	sig, err := update.Sign(data, key)
	if err != nil {
		return err
	}
	return os.WriteFile(args[0]+".sig", []byte(sig+"\n"), 0o644)
}
