package plugins

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemClientCertificateIsAskedWithTheHost(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "Access is denied.", http.StatusForbidden)
			return
		}
		w.Write([]byte("ok"))
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	srv.StartTLS()
	defer srv.Close()
	// The server's own certificate stands in for the system's.
	own := srv.TLS.Certificates[0]
	var asked string
	old := SystemClientCertificate
	defer func() { SystemClientCertificate = old }()
	SystemClientCertificate = func(host string, _ [][]byte) (*tls.Certificate, error) {
		asked = host
		return &own, nil
	}
	base := srv.Client()
	if WithSystemClientCertificate(&http.Client{Transport: base.Transport}).Transport == base.Transport {
		t.Fatal("transport not wrapped")
	}
	res, err := WithSystemClientCertificate(&http.Client{Transport: base.Transport}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || asked != "127.0.0.1" {
		t.Fatalf("%d, asked for %q", res.StatusCode, asked)
	}
	// None from the system: the request goes without one.
	SystemClientCertificate = func(string, [][]byte) (*tls.Certificate, error) { return nil, nil }
	res, err = WithSystemClientCertificate(&http.Client{Transport: base.Transport}).Get(srv.URL)
	if err != nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("%v %v", res, err)
	}
	res.Body.Close()
}
