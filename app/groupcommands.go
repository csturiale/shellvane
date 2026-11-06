package app

import (
	"strings"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
)

// GroupCommands lists what a shared host group would run: commands on this
// computer (ProxyCommand, password, certificate and pre-connect commands),
// commands on the servers with this person's identity (startup and remote
// commands), and X11 forwarding (a server reaching the screen).
func GroupCommands(tg hosts.TemplateGroup) []string {
	var out []string
	var walk func(prefix string, g hosts.TemplateGroup)
	add := func(where, what, c string) {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, where+" · "+what+": "+c)
		}
	}
	check := func(where string, s settings.Settings) {
		add(where, "ProxyCommand", deref(s.ProxyCommand))
		add(where, "password command", deref(s.PasswordCommand))
		add(where, "certificate command", deref(s.CertificateCommand))
		add(where, "before connecting", deref(s.PreConnectCommand))
		add(where, "remote command", deref(s.RemoteCommand))
		for _, c := range s.StartupCommands {
			add(where, "after login", c)
		}
		if s.ForwardX11 != nil && *s.ForwardX11 {
			add(where, "X11", "forwarded")
		}
	}
	walk = func(prefix string, g hosts.TemplateGroup) {
		path := prefix + g.Name
		check(path, g.Settings)
		for _, h := range g.Hosts {
			check(path+" / "+h.Name, h.Settings)
		}
		for _, c := range g.Groups {
			walk(path+" / ", c)
		}
	}
	walk("", tg)
	return out
}

// StripCommands leaves out of a shared group what GroupCommands lists,
// for the copy used until those commands are approved.
func StripCommands(tg hosts.TemplateGroup) hosts.TemplateGroup {
	strip := func(s settings.Settings) settings.Settings {
		s.ProxyCommand, s.PasswordCommand, s.CertificateCommand, s.PreConnectCommand, s.RemoteCommand = nil, nil, nil, nil, nil
		s.StartupCommands = nil
		if s.ForwardX11 != nil && *s.ForwardX11 {
			s.ForwardX11 = nil
		}
		return s
	}
	out := tg
	out.Settings = strip(tg.Settings)
	out.Hosts = make([]hosts.TemplateHost, len(tg.Hosts))
	for i, h := range tg.Hosts {
		h.Settings = strip(h.Settings)
		out.Hosts[i] = h
	}
	out.Groups = make([]hosts.TemplateGroup, len(tg.Groups))
	for i, c := range tg.Groups {
		out.Groups[i] = StripCommands(c)
	}
	return out
}
