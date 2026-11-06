package app

import (
	"bytes"
	"errors"
	"io"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/files"
)

// sshRunner runs the shell file system's commands on their own channels.
type sshRunner struct {
	c *ssh.Client
}

func exitErr(err error, stderr string) error {
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		return &files.ExitError{Status: exit.ExitStatus(), Stderr: stderr}
	}
	return err
}

func (r sshRunner) Run(cmd string) ([]byte, error) {
	sess, err := r.c.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdout, sess.Stderr = &stdout, &stderr
	if err := sess.Run(cmd); err != nil {
		return stdout.Bytes(), exitErr(err, stderr.String())
	}
	return stdout.Bytes(), nil
}

type sessionReader struct {
	io.Reader
	sess *ssh.Session
}

func (s sessionReader) Close() error { return s.sess.Close() }

func (r sshRunner) Stream(cmd string) (io.ReadCloser, error) {
	sess, err := r.c.NewSession()
	if err != nil {
		return nil, err
	}
	out, err := sess.StdoutPipe()
	if err == nil {
		err = sess.Start(cmd)
	}
	if err != nil {
		sess.Close()
		return nil, err
	}
	return sessionReader{out, sess}, nil
}

type sessionWriter struct {
	io.WriteCloser
	sess   *ssh.Session
	stderr *bytes.Buffer
}

// Close ends the input and waits for the command, so errors (disk full,
// permission) are reported.
func (s sessionWriter) Close() error {
	s.WriteCloser.Close()
	err := s.sess.Wait()
	s.sess.Close()
	return exitErr(err, s.stderr.String())
}

func (r sshRunner) Sink(cmd string) (io.WriteCloser, error) {
	sess, err := r.c.NewSession()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	sess.Stderr = &stderr
	in, err := sess.StdinPipe()
	if err == nil {
		err = sess.Start(cmd)
	}
	if err != nil {
		sess.Close()
		return nil, err
	}
	return sessionWriter{in, sess, &stderr}, nil
}
