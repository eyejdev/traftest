package engine

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewHTTPClient creates an optimized HTTP client with connection reuse and keep-alive
func NewHTTPClient(timeout time.Duration, maxConns int) *http.Client {
	if maxConns <= 0 {
		maxConns = 1000
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxConns,
		MaxIdleConnsPerHost:   maxConns,
		MaxConnsPerHost:       0, // No limit per host (we control via workers)
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Useful for self-signed certificates in local dev
		},
		DisableCompression: false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow redirects automatically, record as 3xx response
			return http.ErrUseLastResponse
		},
	}
}
