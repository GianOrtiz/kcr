package checkpoint

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

type CheckpointService interface {
	Checkpoint(podNode, podID, podNamespace, containerName string) error
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
func (s *checkpointService) Checkpoint(podNode, podID, podNamespace, containerName string) error {
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
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		log.Printf("res: %+v", string(body))
		return fmt.Errorf("failed to checkpoint pod: %s", res.Status)
	}

	return nil
}
