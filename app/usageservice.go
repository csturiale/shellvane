package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/version"
)

// ErrUsage is returned when the usage report cannot be sent.
const ErrUsage errs.Code = "usage.failed"

// UsageURL receives the anonymous usage reports (opt-in).
var UsageURL = version.SiteURL + "/api/usage"

// UsageReport is what an opted-in app sends once a week: how many times
// features and well-known command-line tools were used in that period,
// plus the version and the system. No identifier, host, path or argument.
type UsageReport struct {
	Days     int            `json:"days"`
	Features map[string]int `json:"features"`
	Tools    map[string]int `json:"tools"`
}

// UsagePayload is the report as it goes on the wire.
type UsagePayload struct {
	V        int            `json:"v"`
	Version  string         `json:"version"`
	OS       string         `json:"os"`
	Arch     string         `json:"arch"`
	Days     int            `json:"days"`
	Features map[string]int `json:"features"`
	Tools    map[string]int `json:"tools"`
}

// UsageKey is the form of feature and tool names: anything else is left out.
var UsageKey = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,47}$`)

// UsageService sends the weekly report when the person agreed to it; the
// frontend keeps the counters and the consent.
type UsageService struct {
	client *http.Client
}

// NewUsageService returns the service.
func NewUsageService() *UsageService {
	return &UsageService{client: &http.Client{Timeout: 20 * time.Second}}
}

// Payload returns exactly what Send would send, to show it first.
func (s *UsageService) Payload(r UsageReport) UsagePayload {
	clean := func(m map[string]int) map[string]int {
		out := map[string]int{}
		for k, n := range m {
			if UsageKey.MatchString(k) && n > 0 && len(out) < 200 {
				out[k] = min(n, 1_000_000)
			}
		}
		return out
	}
	return UsagePayload{V: 1, Version: version.Version, OS: runtime.GOOS, Arch: runtime.GOARCH,
		Days: max(1, min(r.Days, 366)), Features: clean(r.Features), Tools: clean(r.Tools)}
}

// Send posts the report.
func (s *UsageService) Send(ctx context.Context, r UsageReport) error {
	body, _ := json.Marshal(s.Payload(r))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, UsageURL, bytes.NewReader(body))
	if err != nil {
		return errs.Wrap(err, ErrUsage, errs.Params{"detail": err.Error()})
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	res, err := s.client.Do(req)
	if err != nil {
		return errs.Wrap(err, ErrUsage, errs.Params{"detail": err.Error()})
	}
	res.Body.Close()
	if res.StatusCode/100 != 2 {
		return errs.New(ErrUsage, errs.Params{"detail": fmt.Sprint(res.StatusCode)})
	}
	return nil
}
