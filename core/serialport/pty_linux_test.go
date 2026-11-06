package serialport

import (
	"io"
	"testing"
	"time"

	"github.com/creack/pty"
)

// A pseudo-terminal stands in for a USB serial adapter.
func TestOpenPseudoTerminal(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty:", err)
	}
	defer ptmx.Close()
	name := tty.Name()
	tty.Close()

	line, err := Open(Settings{Port: name, Baud: 115200})
	if err != nil {
		t.Fatal(err)
	}
	defer line.Close()
	ptmx.Write([]byte("U-Boot> "))
	buf := make([]byte, 16)
	done := make(chan string, 1)
	go func() {
		n, _ := io.ReadAtLeast(line, buf, 8)
		done <- string(buf[:n])
	}()
	select {
	case got := <-done:
		if got != "U-Boot> " {
			t.Fatalf("read %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nothing read from the line")
	}
}
