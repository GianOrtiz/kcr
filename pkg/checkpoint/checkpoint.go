package checkpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type CheckpointService interface {
	Checkpoint(podNode, podID, podNamespace, containerName string) (string, error)
}

// CheckpointService is a service to abstract checkpointing of a pod container..
type checkpointService struct {
	client               http.Client
	kubernetesAPIAddress string
}

// New creates a new CheckpointService to work in the given node.
func New(kubernetesAPIAddress string) (CheckpointService, error) {
	return &checkpointService{
		client:               *http.DefaultClient,
		kubernetesAPIAddress: kubernetesAPIAddress,
	}, nil
}

// Checkpoint checkpoints a pod container.
func (s *checkpointService) Checkpoint(podNode, podID, podNamespace, containerName string) (string, error) {
	address := fmt.Sprintf(
		"%s/api/v1/nodes/%s/proxy/checkpoint/%s/%s/%s",
		s.kubernetesAPIAddress,
		podNode,
		podNamespace,
		podID,
		containerName,
	)

	res, err := s.client.Post(address, "application/json", nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var body struct {
		Items []string `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to checkpoint pod: %s", res.Status)
	}

	if len(body.Items) > 0 {
		return body.Items[0], nil
	}

	return "", errors.New("no items in response")
}
