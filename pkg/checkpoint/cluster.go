package checkpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type clusterCheckpointService struct {
	clientset *kubernetes.Clientset
}

// New creates a new CheckpointService to work in the given node for production clusters.
func NewClusterCheckpointService() (CheckpointService, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &clusterCheckpointService{
		clientset: clientset,
	}, nil
}

func (s *clusterCheckpointService) Checkpoint(podNode, podID, podNamespace, containerName string, ctx context.Context) (
	string, error) {
	paths := []string{
		"api",
		"v1",
		"nodes",
		podNode,
		"proxy",
		"checkpoint",
		podNamespace,
		podID,
		containerName,
	}
	result := s.clientset.RESTClient().Post().AbsPath(paths...).Do(ctx)
	if result.Error() != nil {
		return "", result.Error()
	}

	var body struct {
		Items []string `json:"items"`
	}
	rawBody, err := result.Raw()
	if err != nil {
		return "", fmt.Errorf("failed to get raw body: %w", err)
	}

	if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	var statusCode int
	result.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("failed to checkpoint pod: %d", statusCode)
	}

	if len(body.Items) > 0 {
		return body.Items[0], nil
	}

	return "", fmt.Errorf("no items in response")
}
