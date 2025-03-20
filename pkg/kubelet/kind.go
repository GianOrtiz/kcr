package kubelet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SetupKindCertificates extracts certificates from the Kind cluster and prepares
// them for use with the KubeletClient. It returns a directory path containing
// the necessary certificates.
func SetupKindCertificates(clusterName string) (string, error) {
	// Create a temporary directory to store certificates
	tempDir, err := os.MkdirTemp("", "kind-certs-")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	containerName := fmt.Sprintf("%s-control-plane", clusterName)

	// Extract the CA certificate from the Kind cluster
	caCmd := exec.Command("docker", "exec",
		containerName,
		"cat", "/etc/kubernetes/pki/ca.crt")
	caCert, err := caCmd.Output()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("extracting CA certificate: %w", err)
	}
	if err = os.WriteFile(filepath.Join(tempDir, "ca.crt"), caCert, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("writing CA certificate: %w", err)
	}

	clientCrtCmd := exec.Command("docker", "exec",
		containerName,
		"cat", "/etc/kubernetes/pki/apiserver-kubelet-client.crt")
	clientCrt, err := clientCrtCmd.Output()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("extracting client certificate: %w", err)
	}
	if err = os.WriteFile(filepath.Join(tempDir, "client.crt"), clientCrt, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("writing client certificate: %w", err)
	}

	clientKeyCmd := exec.Command("docker", "exec",
		containerName,
		"cat", "/etc/kubernetes/pki/apiserver-kubelet-client.key")
	clientKey, err := clientKeyCmd.Output()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("extracting client certificate: %w", err)
	}
	if err = os.WriteFile(filepath.Join(tempDir, "client.key"), clientKey, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("writing client certificate: %w", err)
	}

	return tempDir, nil
}

// NewForKind creates a KubeletClient configured for a Kind cluster
func NewForKind(clusterName string) (*KubeletClient, error) {
	certDir, err := SetupKindCertificates(clusterName)
	if err != nil {
		return nil, fmt.Errorf("setting up Kind certificates: %w", err)
	}

	// Use the certificate directory with our KindConfig
	config := KindConfig(certDir)

	// Create a client with the Kind configuration
	client, err := NewWithConfig(config)
	if err != nil {
		os.RemoveAll(certDir)
		return nil, err
	}

	return client, nil
}
