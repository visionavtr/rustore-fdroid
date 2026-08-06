package internal

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"errors"
	"net/http"
	"strings"
	"time"
)

const httpClientTimeout = 10 * time.Minute

// Published by Gosuslugi at
// https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt.
//
//go:embed certs/russian_trusted_root_ca.pem
var russianTrustedRootCAPEM []byte

type scopedTrustTransport struct {
	defaultTransport http.RoundTripper
	ruStoreTransport http.RoundTripper
}

func newHTTPClient() *http.Client {
	transport, err := newScopedTrustTransport(
		http.DefaultTransport.(*http.Transport),
		russianTrustedRootCAPEM,
	)
	if err != nil {
		panic(err)
	}

	return &http.Client{
		Timeout:   httpClientTimeout,
		Transport: transport,
	}
}

func newScopedTrustTransport(base *http.Transport, rootPEM []byte) (*scopedTrustTransport, error) {
	defaultTransport := base.Clone()
	ruStoreTransport := base.Clone()

	tlsConfig := ruStoreTransport.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}

	roots := tlsConfig.RootCAs
	if roots != nil {
		roots = roots.Clone()
	} else {
		var err error
		roots, err = x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
	}
	if !roots.AppendCertsFromPEM(rootPEM) {
		return nil, errors.New("invalid Russian Trusted Root CA certificate")
	}

	tlsConfig.RootCAs = roots
	ruStoreTransport.TLSClientConfig = tlsConfig

	return &scopedTrustTransport{
		defaultTransport: defaultTransport,
		ruStoreTransport: ruStoreTransport,
	}, nil
}

func (t *scopedTrustTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if isRuStoreHost(req.URL.Hostname()) {
		return t.ruStoreTransport.RoundTrip(req)
	}
	return t.defaultTransport.RoundTrip(req)
}

func (t *scopedTrustTransport) CloseIdleConnections() {
	if transport, ok := t.defaultTransport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
	if transport, ok := t.ruStoreTransport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func isRuStoreHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return host == "rustore.ru" || strings.HasSuffix(host, ".rustore.ru")
}
