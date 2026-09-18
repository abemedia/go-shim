package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	shim "github.com/abemedia/go-shim"
)

func main() {
	if ca, err := os.ReadFile(os.Getenv("SHIM_TEST_CA")); err == nil {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(ca)
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		}
	}
	shim.Main(shim.Config{URL: os.Getenv("SHIM_TEST_URL"), Version: "v1.0.0"})
}
