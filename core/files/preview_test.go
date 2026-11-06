package files

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakePreview(t *testing.T) {
	dir := t.TempDir()
	fsys := Local{}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	os.WriteFile(filepath.Join(dir, "a.png"), png, 0o600)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello\nwörld\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "bin"), []byte{0x7f, 'E', 'L', 'F', 0, 1, 2}, 0o600)
	os.WriteFile(filepath.Join(dir, "fake.png"), []byte("not an image"), 0o600)
	big := strings.Repeat("é", PreviewTextLimit) // 2 bytes each: cut in the middle
	os.WriteFile(filepath.Join(dir, "big.log"), []byte(big), 0o600)

	cases := map[string]string{"a.png": "image", "notes.txt": "text", "bin": "binary", "fake.png": "text", "big.log": "text"}
	for name, kind := range cases {
		p, err := MakePreview(fsys, filepath.Join(dir, name))
		if err != nil || p.Kind != kind {
			t.Errorf("%s: %+v, %v", name, p.Kind, err)
		}
	}
	p, _ := MakePreview(fsys, filepath.Join(dir, "big.log"))
	if !p.Truncated || p.Size != int64(len(big)) || !strings.HasSuffix(p.Text, "é") {
		t.Errorf("big = truncated %v size %d", p.Truncated, p.Size)
	}
	if _, err := MakePreview(fsys, dir); err == nil {
		t.Error("a directory has no preview")
	}
}
