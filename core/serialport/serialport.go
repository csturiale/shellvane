// Package serialport opens serial consoles (routers, switches, boards) for
// terminal panes.
package serialport

import (
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.bug.st/serial"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrInvalid errs.Code = "serial.invalid"
	ErrOpen    errs.Code = "serial.openFailed"
)

// Settings describe a serial line; the zero values are 9600 8N1.
type Settings struct {
	Port     string `json:"port"`
	Baud     int    `json:"baud"`
	DataBits int    `json:"dataBits"`
	// Parity is "none", "odd", "even", "mark" or "space".
	Parity string `json:"parity"`
	// StopBits is "1", "1.5" or "2".
	StopBits string `json:"stopBits"`
}

// Defaults fills unset fields.
func (s Settings) Defaults() Settings {
	if s.Baud == 0 {
		s.Baud = 9600
	}
	if s.DataBits == 0 {
		s.DataBits = 8
	}
	if s.Parity == "" {
		s.Parity = "none"
	}
	if s.StopBits == "" {
		s.StopBits = "1"
	}
	return s
}

// Label is how the line is shown: "/dev/ttyUSB0 115200 8N1".
func (s Settings) Label() string {
	s = s.Defaults()
	stop := s.StopBits
	return s.Port + " " + strconv.Itoa(s.Baud) + " " + strconv.Itoa(s.DataBits) + strings.ToUpper(s.Parity[:1]) + stop
}

// Parse reads "serial://PORT?baud=115200&data=8&parity=none&stop=1"; on
// Unix the port is a path ("serial:///dev/ttyUSB0"), on Windows a name
// ("serial://COM3").
func Parse(spec string) (Settings, error) {
	bad := errs.New(ErrInvalid, errs.Params{"spec": spec})
	rest, ok := strings.CutPrefix(spec, "serial://")
	if !ok {
		return Settings{}, bad
	}
	port, query, _ := strings.Cut(rest, "?")
	port, err := url.PathUnescape(port)
	if err != nil || strings.TrimSpace(port) == "" {
		return Settings{}, bad
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return Settings{}, bad
	}
	s := Settings{Port: port, Parity: strings.ToLower(q.Get("parity")), StopBits: q.Get("stop")}
	for key, dst := range map[string]*int{"baud": &s.Baud, "data": &s.DataBits} {
		if v := q.Get(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return Settings{}, bad
			}
			*dst = n
		}
	}
	s = s.Defaults()
	if _, ok := parities[s.Parity]; !ok || s.DataBits < 5 || s.DataBits > 8 {
		return Settings{}, bad
	}
	if _, ok := stopBits[s.StopBits]; !ok {
		return Settings{}, bad
	}
	return s, nil
}

// Spec renders settings as a serial:// spec that Parse reads back.
func (s Settings) Spec() string {
	s = s.Defaults()
	q := url.Values{}
	q.Set("baud", strconv.Itoa(s.Baud))
	if s.DataBits != 8 {
		q.Set("data", strconv.Itoa(s.DataBits))
	}
	if s.Parity != "none" {
		q.Set("parity", s.Parity)
	}
	if s.StopBits != "1" {
		q.Set("stop", s.StopBits)
	}
	return "serial://" + s.Port + "?" + q.Encode()
}

var parities = map[string]serial.Parity{
	"none": serial.NoParity, "odd": serial.OddParity, "even": serial.EvenParity,
	"mark": serial.MarkParity, "space": serial.SpaceParity,
}

var stopBits = map[string]serial.StopBits{
	"1": serial.OneStopBit, "1.5": serial.OnePointFiveStopBits, "2": serial.TwoStopBits,
}

// Ports lists the serial ports of this computer.
func Ports() []string {
	list, err := serial.GetPortsList()
	if err != nil || list == nil {
		return []string{}
	}
	sort.Strings(list)
	return list
}

// Line is an open serial port, used as a pane's terminal.
type Line struct {
	port serial.Port
	once sync.Once
	done chan struct{}
}

// Open opens the port.
func Open(s Settings) (*Line, error) {
	s = s.Defaults()
	p, err := serial.Open(s.Port, &serial.Mode{
		BaudRate: s.Baud, DataBits: s.DataBits, Parity: parities[s.Parity], StopBits: stopBits[s.StopBits],
	})
	if err != nil {
		return nil, errs.Wrap(err, ErrOpen, errs.Params{"port": s.Port})
	}
	return &Line{port: p, done: make(chan struct{})}, nil
}

// Read returns what the device sends.
func (l *Line) Read(p []byte) (int, error) {
	n, err := l.port.Read(p)
	if err != nil || n == 0 {
		// A zero read means the port was closed (the device was unplugged).
		l.end()
		if err == nil {
			err = io.EOF
		}
	}
	return n, err
}

// Write sends keystrokes.
func (l *Line) Write(p []byte) (int, error) { return l.port.Write(p) }

// Resize does nothing: a serial line has no window size.
func (l *Line) Resize(int, int) error { return nil }

// Wait blocks until the port is closed; there is no exit status.
func (l *Line) Wait() int {
	<-l.done
	return 0
}

// Close closes the port.
func (l *Line) Close() error {
	err := l.port.Close()
	l.end()
	return err
}

func (l *Line) end() { l.once.Do(func() { close(l.done) }) }
