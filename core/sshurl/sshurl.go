// Package sshurl parses ssh:// and sftp:// links (RFC draft
// "SSH URI": ssh://[user[;params]@]host[:port], sftp:// adds a path).
package sshurl

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrInvalid reports a link that is not a usable ssh:// or sftp:// URL.
const ErrInvalid errs.Code = "url.invalid"

// Link is a parsed ssh:// or sftp:// URL.
type Link struct {
	Scheme string `json:"scheme"` // "ssh" or "sftp"
	User   string `json:"user"`
	Host   string `json:"host"`
	Port   int    `json:"port"` // 0 when not given
	// Path is the folder or file of an sftp:// link ("" otherwise).
	Path string `json:"path"`
}

// Spec is the link as user@host:port for a quick connection.
func (l Link) Spec() string {
	s := l.Host
	if strings.Contains(s, ":") {
		s = "[" + s + "]"
	}
	if l.Port != 0 {
		s += ":" + strconv.Itoa(l.Port)
	}
	if l.User != "" {
		s = l.User + "@" + s
	}
	return s
}

// Parse reads a link. Connection parameters after ';' in the user part
// (such as a host key fingerprint) are ignored.
func Parse(raw string) (Link, error) {
	bad := errs.New(ErrInvalid, errs.Params{"url": raw})
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Link{}, bad
	}
	l := Link{Scheme: strings.ToLower(u.Scheme)}
	if l.Scheme != "ssh" && l.Scheme != "sftp" {
		return Link{}, bad
	}
	if u.User != nil {
		user, _, _ := strings.Cut(u.User.Username(), ";")
		l.User = user
	}
	l.Host = u.Hostname()
	if l.Host == "" || strings.ContainsAny(l.Host, " /") {
		return Link{}, bad
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return Link{}, bad
		}
		if n != 22 {
			l.Port = n
		}
	}
	if l.Scheme == "sftp" && u.Path != "" && u.Path != "/" {
		l.Path = u.Path
		// sftp://host/~/dir is relative to the home folder.
		if strings.HasPrefix(l.Path, "/~") {
			l.Path = strings.TrimPrefix(l.Path, "/")
		}
	}
	if net.ParseIP(l.Host) == nil && strings.ContainsAny(l.Host, "[]") {
		return Link{}, bad
	}
	return l, nil
}

// Is tells whether s looks like a link this package reads.
func Is(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "ssh://") || strings.HasPrefix(s, "sftp://")
}
