package plugins

import (
	"context"
	"crypto/tls"
	"net/http"
)

// SystemClientCertificate, set by the app where the system keeps
// certificates with their keys (the macOS keychain), answers a server's
// request for a client certificate during a plugin's HTTPS call: host is
// the server's name, acceptableCAs the authorities it accepts (DER
// distinguished names, empty: any). nil, nil sends none.
var SystemClientCertificate func(host string, acceptableCAs [][]byte) (*tls.Certificate, error)

type certHostKey struct{}

// hostTransport puts the request's host in its context, where the TLS
// handshake's request for a certificate finds it.
type hostTransport struct{ next *http.Transport }

func (t hostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.next.RoundTrip(r.WithContext(context.WithValue(r.Context(), certHostKey{}, r.URL.Hostname())))
}

// WithSystemClientCertificate makes c answer a server that asks for a
// client certificate as a browser on this computer does: with the
// system's certificate that the server accepts (the device certificate
// of a company Mac, a smart card's). c is returned as it is when the
// system has no store or its transport is not the standard one.
func WithSystemClientCertificate(c *http.Client) *http.Client {
	tr, ok := c.Transport.(*http.Transport)
	if SystemClientCertificate == nil || !ok {
		return c
	}
	tr = tr.Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	}
	tr.TLSClientConfig.GetClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		host, _ := cri.Context().Value(certHostKey{}).(string)
		cert, err := SystemClientCertificate(host, cri.AcceptableCAs)
		if err != nil || cert == nil {
			return &tls.Certificate{}, nil // none: the server decides
		}
		return cert, nil
	}
	return &http.Client{Transport: hostTransport{tr}, Timeout: c.Timeout, Jar: c.Jar, CheckRedirect: c.CheckRedirect}
}

// WebLoginStop is the key accept sets in the values it was given to end
// a sign-in window without success (the caller has the reason): the
// window closes instead of going on.
const WebLoginStop = "__stop"
