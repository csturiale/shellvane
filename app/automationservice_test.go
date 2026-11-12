package app

import "testing"

func TestOneLine(t *testing.T) {
	if got := oneLine("kubectl -n x \\\n  delete pod y\n", 100); got != "kubectl -n x \\ ⏎ delete pod y" {
		t.Fatalf("%q", got)
	}
	if got := oneLine("abcdef", 3); got != "abc…" {
		t.Fatalf("%q", got)
	}
}
