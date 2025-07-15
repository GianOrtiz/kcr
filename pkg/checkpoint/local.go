package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type localCheckpointService struct {
	client               http.Client
	kubernetesAPIAddress string
}

// New creates a new CheckpointService to work in the given local environment.
func NewLocalCheckpointService(kubernetesAPIAddress string) (CheckpointService, error) {
	return &localCheckpointService{
		kubernetesAPIAddress: kubernetesAPIAddress,
		client:               *http.DefaultClient,
	}, nil
}

func (s *localCheckpointService) Checkpoint(podNode, podID, podNamespace, containerName string, ctx context.Context) (
	string, error) {
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
	defer func() {
		if err := res.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %s", err.Error())
		}
	}()

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
