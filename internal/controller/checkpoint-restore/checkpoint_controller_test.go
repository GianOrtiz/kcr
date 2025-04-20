/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package checkpointrestore

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
)

var _ = Describe("Checkpoint Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		checkpoint := &checkpointrestorev1.Checkpoint{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Checkpoint")
			err := k8sClient.Get(ctx, typeNamespacedName, checkpoint)
			if err != nil && errors.IsNotFound(err) {
				now := metav1.Now()
				resource := &checkpointrestorev1.Checkpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Status: checkpointrestorev1.CheckpointStatus{
						CheckpointImage:    "image-reference",
						Phase:              "Created",
						LastTransitionTime: &now,
					},
					Spec: checkpointrestorev1.CheckpointSpec{
						CheckpointData: "/var/lib/kubelet/checkpoints/checkpoint-kcr-example-5b9845566-rhnj2_default-kcr-example-1744851420.tar",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &checkpointrestorev1.Checkpoint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Checkpoint")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource when the image builder succeeds", func() {
			By("Reconciling the created resource")
			imageBuilder := mockImageBuilder{mockedResult: nil}
			controllerReconciler := &CheckpointReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				ImageBuilder: &imageBuilder,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			var checkpoint checkpointrestorev1.Checkpoint
			Expect(k8sClient.Get(ctx, typeNamespacedName, &checkpoint)).To(Succeed())
			Expect(checkpoint.Status.Phase).To(Equal("ImageBuilt"))
		})

		It("should fail to reconcile the resource when the image builder fails", func() {
			By("Reconciling the created resource")
			imageBuilder := mockImageBuilder{mockedResult: fmt.Errorf("mocked error")}
			controllerReconciler := &CheckpointReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				ImageBuilder: &imageBuilder,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			var checkpoint checkpointrestorev1.Checkpoint
			Expect(k8sClient.Get(ctx, typeNamespacedName, &checkpoint)).To(Succeed())
			Expect(checkpoint.Status.Phase).To(Equal("Failed"))
		})
	})
})
