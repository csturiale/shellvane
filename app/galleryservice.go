package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/update"
	"github.com/csturiale/shellvane/core/version"
)

// Gallery errors.
const (
	// ErrGallery: the gallery cannot be read.
	ErrGallery errs.Code = "gallery.unavailable"
	// ErrGallerySignature: the gallery's index is not signed by the project.
	ErrGallerySignature errs.Code = "gallery.signature"
	// ErrGalleryItem: the file is not in the index, or does not match it.
	ErrGalleryItem errs.Code = "gallery.item"
)

// GalleryURL is the community gallery's index on the website.
var GalleryURL = version.SiteURL + "/gallery/index.json"

// galleryVerify checks the index's signature; tests replace it.
var galleryVerify = update.Verify

// GalleryIndex lists the snippet collections and color schemes shared on
// the website.
type GalleryIndex struct {
	Snippets []GallerySnippets `json:"snippets"`
	Themes   []GalleryTheme    `json:"themes"`
}

// GallerySnippets is a collection of snippets.
type GallerySnippets struct {
	File   string   `json:"file"`
	Name   string   `json:"name"`
	Count  int      `json:"count"`
	Names  []string `json:"names"`
	SHA256 string   `json:"sha256"`
}

// GalleryTheme is a terminal color scheme.
type GalleryTheme struct {
	File   string   `json:"file"`
	Name   string   `json:"name"`
	Colors []string `json:"colors"`
	SHA256 string   `json:"sha256"`
}

// GalleryResult tells what an install added.
type GalleryResult struct {
	Added int      `json:"added"`
	Names []string `json:"names"`
}

// GalleryService installs snippets and color schemes from the website's
// community gallery. Its index is signed with the release key and lists
// each file's SHA-256, so the website alone cannot change what installs.
type GalleryService struct {
	snippets *SnippetService
	themes   *ThemeService
	client   *http.Client
}

// NewGalleryService returns the service.
func NewGalleryService(sn *SnippetService, th *ThemeService) *GalleryService {
	return &GalleryService{snippets: sn, themes: th, client: &http.Client{Timeout: 30 * time.Second}}
}

// Gallery reads and verifies the index.
func (s *GalleryService) Gallery(ctx context.Context) (GalleryIndex, error) {
	body, err := s.get(ctx, GalleryURL, 2<<20)
	if err != nil {
		return GalleryIndex{}, err
	}
	sig, err := s.get(ctx, GalleryURL+".sig", 4096)
	if err != nil || !galleryVerify(body, sig) {
		return GalleryIndex{}, errs.New(ErrGallerySignature, nil)
	}
	var idx GalleryIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return GalleryIndex{}, errs.Wrap(err, ErrGallery, errs.Params{"detail": err.Error()})
	}
	return idx, nil
}

// Install adds a collection of snippets or a color scheme: the file must be
// in the (verified) index and match its SHA-256.
func (s *GalleryService) Install(ctx context.Context, kind, file string) (GalleryResult, error) {
	idx, err := s.Gallery(ctx)
	if err != nil {
		return GalleryResult{}, err
	}
	sum := ""
	switch kind {
	case "snippets":
		for _, x := range idx.Snippets {
			if x.File == file {
				sum = x.SHA256
			}
		}
	case "themes":
		for _, x := range idx.Themes {
			if x.File == file {
				sum = x.SHA256
			}
		}
	}
	if len(sum) != 64 || !strings.HasPrefix(file, "/gallery/") || strings.Contains(file, "..") {
		return GalleryResult{}, errs.New(ErrGalleryItem, errs.Params{"file": file})
	}
	base := strings.TrimSuffix(GalleryURL, "/gallery/index.json")
	data, err := s.get(ctx, base+file, 1<<20)
	if err != nil {
		return GalleryResult{}, err
	}
	got := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(got[:]), sum) {
		return GalleryResult{}, errs.New(ErrGalleryItem, errs.Params{"file": file})
	}
	name := path.Base(file)
	if kind == "snippets" {
		res, err := s.snippets.Import(ctx, name, string(data))
		return GalleryResult{Added: res.Added}, err
	}
	saved, err := s.themes.Import(ctx, name, string(data))
	out := GalleryResult{Added: len(saved)}
	for _, t := range saved {
		out.Names = append(out.Names, t.Name)
	}
	return out, err
}

func (s *GalleryService) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://127.0.0.1") {
		return nil, errs.New(ErrGallery, errs.Params{"detail": "https required"})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errs.Wrap(err, ErrGallery, errs.Params{"detail": err.Error()})
	}
	req.Header.Set("User-Agent", version.UserAgent())
	res, err := s.client.Do(req)
	if err != nil {
		return nil, errs.Wrap(err, ErrGallery, errs.Params{"detail": err.Error()})
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errs.New(ErrGallery, errs.Params{"detail": fmt.Sprint(res.StatusCode)})
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errs.New(ErrGallery, errs.Params{"detail": "too large"})
	}
	return data, nil
}
