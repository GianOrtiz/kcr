package checkpoint

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/GianOrtiz/kcr/pkg/kubelet"
)

type CheckpointService interface {
	Checkpoint(podNode, podID, podNamespace, containerName string) error
}

// CheckpointService is a service to abstract checkpointing of a pod container..
type checkpointService struct {
	kubeletClient  *kubelet.KubeletClient
	masterNodeIP   string
	masterNodePort int
}

// New creates a new CheckpointService to work in the given node.
func New(masterNodeIP string, masterNodePort int) (CheckpointService, error) {
	kubeletClient, err := kubelet.NewForKind("kind")
	if err != nil {
		return nil, err
	}
	return &checkpointService{
		kubeletClient:  kubeletClient,
		masterNodeIP:   masterNodeIP,
		masterNodePort: masterNodePort,
	}, nil
}

// Checkpoint checkpoints a pod container.
func (s *checkpointService) Checkpoint(podNode, podID, podNamespace, containerName string) error {
	address := fmt.Sprintf(
		"https://%s:%d/api/v1/nodes/%s/proxy/checkpoint/%s/%s/%s",
		s.masterNodeIP,
		s.masterNodePort,
		podNode,
		podNamespace,
		podID,
		containerName,
	)
	log.Println("address:", address)

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
