package version

import (
	"regexp"
	"testing"
)

func TestUserAgentHasOnlyVersionAndPlatform(t *testing.T) {
	if ua := UserAgent(); !regexp.MustCompile(`^Shellvane/\S+ \([a-z0-9]+/[a-z0-9]+\)$`).MatchString(ua) {
		t.Fatalf("UserAgent() = %q", ua)
	}
}
