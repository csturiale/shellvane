// Package termtext turns terminal output into lines of plain text:
// escape sequences are removed, a carriage return not followed by a
// newline starts the line again (progress bars), backspace erases.
package termtext

// MaxLine bounds the text kept for one line; longer lines are cut (the
// rest of the line is dropped until the next newline).
const MaxLine = 4096

// Cleaner splits output into plain lines.
type Cleaner struct {
	line []byte
	esc  escState
	cr   bool
	// OnReset is called when the current line is discarded or completed.
	OnReset func()
}

type escState int

const (
	escNone escState = iota
	escStart
	escCSI
	escString // OSC, DCS...: until BEL or ESC \
	escStringEnd
)

// Feed consumes output; line is called with every completed line.
func (c *Cleaner) Feed(data []byte, line func([]byte)) {
	for _, b := range data {
		switch c.esc {
		case escStart:
			switch b {
			case '[':
				c.esc = escCSI
			case ']', 'P', '_', '^', 'X':
				c.esc = escString
			default:
				c.esc = escNone // two-byte sequence
			}
			continue
		case escCSI:
			if b >= 0x40 && b <= 0x7e {
				c.esc = escNone
			}
			continue
		case escString:
			if b == 0x07 {
				c.esc = escNone
			} else if b == 0x1b {
				c.esc = escStringEnd
			}
			continue
		case escStringEnd:
			c.esc = escNone
			continue
		}
		if c.cr && b != '\n' {
			c.reset()
		}
		c.cr = false
		switch {
		case b == 0x1b:
			c.esc = escStart
		case b == '\n':
			line(c.line)
			c.reset()
		case b == '\r':
			c.cr = true
		case b == '\b':
			if len(c.line) > 0 {
				c.line = c.line[:len(c.line)-1]
			}
		case b < 0x20 && b != '\t':
			// other controls carry no text
		case len(c.line) < MaxLine:
			c.line = append(c.line, b)
		}
	}
}

// Partial returns the line being written, outside escape sequences.
func (c *Cleaner) Partial() []byte {
	if c.esc != escNone {
		return nil
	}
	return c.line
}

func (c *Cleaner) reset() {
	c.line = c.line[:0]
	if c.OnReset != nil {
		c.OnReset()
	}
}
