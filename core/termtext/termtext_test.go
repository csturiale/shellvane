package termtext

import (
	"strings"
	"testing"
)

func TestCleaner(t *testing.T) {
	var lines []string
	c := &Cleaner{}
	add := func(b []byte) { lines = append(lines, string(b)) }
	c.Feed([]byte("\x1b]0;title\x07\x1b[31mred\x1b[0m text\r\n10%\r50%\rdone\nab\bc"), add)
	if strings.Join(lines, "|") != "red text|done" || string(c.Partial()) != "ac" {
		t.Fatalf("lines %q partial %q", lines, c.Partial())
	}
	c.Feed([]byte("\x1b["), add)
	if c.Partial() != nil {
		t.Fatal("inside an escape sequence")
	}
}
