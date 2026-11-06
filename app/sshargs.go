package app

import (
	"context"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
)

// SSHArgs builds the system ssh's arguments for a saved host (by name or
// ID), a link or user@host: port, user, jump hosts (-J), ProxyCommand and
// agent forwarding.
func SSHArgs(ctx context.Context, repo *hosts.Repo, target string) ([]string, error) {
	req, err := ResolveOpen(ctx, repo, target)
	if err != nil {
		return nil, err
	}
	if req.Kind == "target" {
		if strings.HasPrefix(req.Spec, "-") {
			return nil, errs.New(hosts.ErrHostnameInvalid, errs.Params{"hostname": req.Spec})
		}
		return []string{req.Spec}, nil
	}
	dest := func(id string) (string, string, error) {
		h, err := repo.Host(ctx, id)
		if err != nil {
			return "", "", err
		}
		res, err := repo.Resolve(ctx, h)
		if err != nil {
			return "", "", err
		}
		addr := h.Hostname
		if res.Settings.User != nil && *res.Settings.User != "" {
			addr = *res.Settings.User + "@" + addr
		}
		port := ""
		if res.Settings.Port != nil {
			port = strconv.Itoa(*res.Settings.Port)
		}
		return addr, port, nil
	}
	h, err := repo.Host(ctx, req.HostID)
	if err != nil {
		return nil, err
	}
	res, err := repo.Resolve(ctx, h)
	if err != nil {
		return nil, err
	}
	addr, port, err := dest(h.ID)
	if err != nil {
		return nil, err
	}
	var args []string
	if port != "" && port != "22" {
		args = append(args, "-p", port)
	}
	if len(res.Settings.JumpChain) > 0 {
		var hops []string
		for _, id := range res.Settings.JumpChain {
			a, p, err := dest(id)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(a, "-") || strings.Contains(a, ",") {
				return nil, errs.New(hosts.ErrHostnameInvalid, errs.Params{"hostname": a})
			}
			if p != "" && p != "22" {
				a += ":" + p
			}
			hops = append(hops, a)
		}
		args = append(args, "-J", strings.Join(hops, ","))
	} else if res.Settings.ProxyCommand != nil && *res.Settings.ProxyCommand != "" {
		args = append(args, "-o", "ProxyCommand="+*res.Settings.ProxyCommand)
	}
	if res.Settings.ForwardAgent != nil && *res.Settings.ForwardAgent {
		args = append(args, "-A")
	}
	// A destination starting with '-' would be read as an option of ssh.
	if strings.HasPrefix(addr, "-") {
		return nil, errs.New(hosts.ErrHostnameInvalid, errs.Params{"hostname": addr})
	}
	return append(args, addr), nil
}
