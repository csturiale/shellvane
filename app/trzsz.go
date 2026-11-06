package app

import (
	"io"
	"sync"

	"github.com/trzsz/trzsz-go/trzsz"
)

// trzszProcess runs a terminal through a trzsz filter: `trz` and `tsz` on
// the server upload and download files, with the system's own file and
// folder dialogs (the webview cannot choose a folder). ZMODEM (rz/sz) stays
// with the terminal's own handler.
type trzszProcess struct {
	inner  terminalProcess
	filter *trzsz.TrzszFilter
	in     *io.PipeWriter // what the person types, into the filter
	out    *io.PipeReader // what the filter shows, to the terminal
	once   sync.Once
}

// withTrzsz wraps an SSH terminal; cols is its width, for the progress bar.
func withTrzsz(p terminalProcess, cols int) terminalProcess {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t := &trzszProcess{inner: p, in: inW, out: outR}
	t.filter = trzsz.NewTrzszFilter(inR, outW, serverSide{p}, p, trzsz.TrzszOptions{TerminalColumns: int32(cols)})
	return t
}

func (t *trzszProcess) Read(b []byte) (int, error)  { return t.out.Read(b) }
func (t *trzszProcess) Write(b []byte) (int, error) { return t.in.Write(b) }
func (t *trzszProcess) Wait() int                   { return t.inner.Wait() }

func (t *trzszProcess) Resize(cols, rows int) error {
	t.filter.SetTerminalColumns(int32(cols))
	return t.inner.Resize(cols, rows)
}

func (t *trzszProcess) Close() error {
	t.once.Do(func() {
		t.filter.Close()
		t.in.Close()
		t.out.Close()
	})
	return t.inner.Close()
}

// serverSide is the terminal as the filter's server: closing it is left to
// trzszProcess.Close.
type serverSide struct{ p terminalProcess }

func (s serverSide) Write(b []byte) (int, error) { return s.p.Write(b) }
func (s serverSide) Close() error                { return nil }
