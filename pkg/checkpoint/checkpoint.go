package checkpoint

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/GianOrtiz/kcr/pkg/kubelet"
)

// CheckpointService is a service to abstract checkpointing of a pod container..
type CheckpointService struct {
	kubeletClient *kubelet.KubeletClient
	nodeIP        string
	nodePort      int
}

// New creates a new CheckpointService to work in the given node.
// TODO: we should build this value when checkpointing the pod from the Node metadata.
func New(nodeIP string, nodePort int) (*CheckpointService, error) {
	kubeletClient, err := kubelet.NewForKind("kind")
	if err != nil {
		return nil, err
	}
	return &CheckpointService{
		kubeletClient: kubeletClient,
		nodeIP:        nodeIP,
		nodePort:      nodePort,
	}, nil
}

// Checkpoint checkpoints a pod container.
func (s *CheckpointService) Checkpoint(podID, podNamespace, containerName string) error {
	address := fmt.Sprintf(
		"https://%s:%d/checkpoint/%s/%s/%s",
		s.nodeIP,
		s.nodePort,
		podNamespace,
		podID,
		containerName,
	)

	res, err := s.kubeletClient.Post(address, "application/json", nil)
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
