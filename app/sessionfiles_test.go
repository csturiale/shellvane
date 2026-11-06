package app

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
)

// fakeProcess records keystrokes and returns canned output.
type fakeProcess struct {
	out     *bytes.Reader
	written bytes.Buffer
}

func (f *fakeProcess) Read(p []byte) (int, error)  { return f.out.Read(p) }
func (f *fakeProcess) Write(p []byte) (int, error) { return f.written.Write(p) }
func (f *fakeProcess) Resize(int, int) error       { return nil }
func (f *fakeProcess) Wait() int                   { return 0 }
func (f *fakeProcess) Close() error                { return nil }

func TestCharset(t *testing.T) {
	enc, err := charset("ISO-8859-1")
	if err != nil || enc == nil {
		t.Fatal(err)
	}
	p := &fakeProcess{out: bytes.NewReader([]byte{'c', 'a', 'f', 0xe9})}
	sh := withCharset(p, enc)
	buf := make([]byte, 16)
	n, _ := sh.Read(buf)
	if string(buf[:n]) != "café" {
		t.Fatalf("decoded %q", buf[:n])
	}
	sh.Write([]byte("è日")) // WHATWG latin1 is windows-1252, which has € but not 日
	if got := p.written.Bytes(); !bytes.Equal(got, []byte{0xe8, '?'}) {
		t.Fatalf("encoded %v", got)
	}
	if enc, _ := charset("utf-8"); enc != nil {
		t.Fatal("UTF-8 needs no conversion")
	}
	if _, err := charset("klingon"); err == nil {
		t.Fatal("unknown encoding accepted")
	}
}

func TestLocalSessionLogAndRecording(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	ctx := context.Background()
	svc := NewSessionService(nil, nil, nil, nil, func(string, any) {})
	EnableLocal(svc, t.TempDir())
	info, err := svc.OpenLocal(ctx, 80, 24, LocalOptions{Program: "/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := svc.get(info.ID)
	<-sess.ready
	logPath, err := svc.StartLogging(info.ID, "text")
	if err != nil {
		t.Fatal(err)
	}
	castPath, err := svc.StartRecording(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The stream handler feeds the capture; do it by hand here.
	sess.capture.write([]byte("\x1b[1mhello\x1b[0m\r\n"))
	svc.Resize(info.ID, 100, 30)
	svc.Close(info.ID)

	files, err := svc.SessionFiles()
	if err != nil || len(files) != 2 {
		t.Fatalf("files %+v, %v", files, err)
	}
	text, _ := svc.ReadSessionFile(logPath)
	if text != "hello\n" {
		t.Fatalf("log %q", text)
	}
	cast, _ := svc.ReadSessionFile(castPath)
	if !strings.Contains(cast, `"o","\u001b[1mhello`) || !strings.Contains(cast, `"r","100x30"`) {
		t.Fatalf("cast %s", cast)
	}
	if err := svc.DeleteSessionFile(logPath); err != nil {
		t.Fatal(err)
	}
}
