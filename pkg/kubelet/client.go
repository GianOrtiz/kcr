package kubelet

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// ClientConfig contains client configuration for the KubeletClient.
type ClientConfig struct {
	// ClientCertDir is the directory containing client.crt and client.key that will be used to authenticate to the Kubelet.
	ClientCertDir string
	// CAFile is the path to the CA certificate that will be used to verify the server certificate.
	CAFile string
	// SkipTLSVerify determines whether to verify the server certificate.
	SkipTLSVerify bool
}

// DefaultConfig returns a config with standard paths for in-cluster usage.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		ClientCertDir: "/var/run/secrets/kubelet-certs",
		CAFile:        "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		SkipTLSVerify: false,
	}
}

// KindConfig returns a configuration suitable for local Kind clusters. This configuration will work with NewWithConfig in order
// to generate the client certificates to authenticate to the Kubelet.
func KindConfig(kindCertDir string) *ClientConfig {
	return &ClientConfig{
		ClientCertDir: kindCertDir,
		CAFile:        filepath.Join(kindCertDir, "ca.crt"),
		SkipTLSVerify: true,
	}
}

type KubeletClient struct {
	http.Client
}

// NewWithConfig creates a KubeletClient with the provided configuration.
func NewWithConfig(config *ClientConfig) (*KubeletClient, error) {
	clientCert, err := tls.LoadX509KeyPair(
		filepath.Join(config.ClientCertDir, "client.crt"),
		filepath.Join(config.ClientCertDir, "client.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("loading client certificates: %w", err)
	}

	certs := x509.NewCertPool()
	pemData, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}

	if !certs.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
			RootCAs:            certs,
			Certificates:       []tls.Certificate{clientCert},
		},
	}

	return &KubeletClient{
		Client: http.Client{
			Transport: tr,
		},
	}, nil
}

// New creates a KubeletClient with the default configuration.
func New() (*KubeletClient, error) {
	return NewWithConfig(DefaultConfig())
}
