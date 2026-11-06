package files

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/csturiale/shellvane/core/errs"
)

// Preview limits.
const (
	PreviewTextLimit  = 512 << 10
	PreviewImageLimit = 10 << 20
)

// Preview is a file's beginning, for a quick look.
type Preview struct {
	Kind      string `json:"kind"` // "text", "image" or "binary"
	MIME      string `json:"mime"`
	Text      string `json:"text"`      // text files
	Data      string `json:"data"`      // images, base64
	Size      int64  `json:"size"`      // whole file
	Truncated bool   `json:"truncated"` // only the beginning is shown
}

var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".bmp": "image/bmp", ".ico": "image/x-icon", ".avif": "image/avif",
}

// MakePreview reads the beginning of a file and tells what it is. SVG is
// shown as text: as an image it could run scripts.
func MakePreview(fsys FS, p string) (Preview, error) {
	e, err := fsys.Stat(p)
	if err != nil {
		return Preview{}, err
	}
	if e.IsDir {
		return Preview{}, errs.New(ErrFailed, errs.Params{"path": p})
	}
	pv := Preview{Size: e.Size}
	limit := int64(PreviewTextLimit)
	mime, isImage := imageTypes[strings.ToLower(path.Ext(p))]
	if isImage {
		limit = PreviewImageLimit
	}
	r, err := fsys.Open(p)
	if err != nil {
		return Preview{}, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return Preview{}, errs.Wrap(err, ErrFailed, errs.Params{"path": p})
	}
	if int64(len(data)) > limit {
		data, pv.Truncated = data[:limit], true
	}
	sniffed := http.DetectContentType(data)
	switch {
	case isImage && !pv.Truncated && strings.HasPrefix(sniffed, "image/"):
		pv.Kind, pv.MIME, pv.Data = "image", mime, base64.StdEncoding.EncodeToString(data)
	case looksText(data):
		pv.Kind, pv.MIME = "text", "text/plain"
		// Do not cut a character in two at the limit.
		for len(data) > 0 && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
		pv.Text = string(data)
	default:
		pv.Kind, pv.MIME = "binary", sniffed
	}
	return pv, nil
}

// looksText: no NUL byte and valid UTF-8 (allowing a cut character at the end).
func looksText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	if len(b) > 4 {
		return utf8.Valid(b[:len(b)-4]) || utf8.Valid(b)
	}
	return utf8.Valid(b)
}
