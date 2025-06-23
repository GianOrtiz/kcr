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
	"errors"
	"time"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	"github.com/GianOrtiz/kcr/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("CheckpointRequest Controller", func() {
	const (
		requestName   = "test-request"
		podName       = "test-pod"
		containerName = "test-container"
	)

	Context("When reconciling a CheckpointRequest", func() {
		var (
			controller        *CheckpointRequestReconciler
			ctx               context.Context
			pod               *corev1.Pod
			namespace         string
			checkpointService *mockCheckpointService
		)

		BeforeEach(func() {
			checkpointService = &mockCheckpointService{
				mockedResultError:  nil,
				mockedResultString: "",
			}

			ctx = context.Background()
			namespace = "ns-" + util.RandStringRunes(5)
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})).To(Succeed())

			// Create a test pod
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: "test-image",
						},
					},
				},
			}

			// Create the fake client with the scheme
			s := runtime.NewScheme()
			Expect(checkpointrestorev1.AddToScheme(s)).To(Succeed())

			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			// Create our test controller with the fake client and our mock service
			controller = &CheckpointRequestReconciler{
				Client:            k8sClient,
				Scheme:            s,
				CheckpointService: checkpointService,
			}
		})

		Describe("When the CheckpointRequest is in Completed status", func() {
			It("should not update the status", func() {
				checkpointRequest := checkpointrestorev1.CheckpointRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      requestName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointRequestSpec{
						PodReference: checkpointrestorev1.PodReference{
							Name:      podName,
							Namespace: namespace,
						},
						ContainerName: containerName,
					},
				}
				Expect(k8sClient.Create(ctx, &checkpointRequest)).To(Succeed())

				checkpointRequest.Status = checkpointrestorev1.CheckpointRequestStatus{
					Phase: "Completed",
				}
				Expect(k8sClient.Status().Update(ctx, &checkpointRequest)).To(Succeed())

				result, err := controller.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      requestName,
						Namespace: namespace,
					},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeZero())

				// Check that the request status was not updated
				updatedRequest := &checkpointrestorev1.CheckpointRequest{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: requestName, Namespace: namespace}, updatedRequest)).To(Succeed())
				Expect(updatedRequest.Status.Phase).To(Equal("Completed"))
			})
		})

		Describe("When the CheckpointRequest is in Failed status", func() {
			It("should not update the status", func() {
				checkpointRequest := checkpointrestorev1.CheckpointRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      requestName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointRequestSpec{
						PodReference: checkpointrestorev1.PodReference{
							Name:      podName,
							Namespace: namespace,
						},
						ContainerName: containerName,
					},
				}
				Expect(k8sClient.Create(ctx, &checkpointRequest)).To(Succeed())

				checkpointRequest.Status = checkpointrestorev1.CheckpointRequestStatus{
					Phase: "Failed",
				}
				Expect(k8sClient.Status().Update(ctx, &checkpointRequest)).To(Succeed())
				result, err := controller.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      requestName,
						Namespace: namespace,
					},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeZero())

				// Check that the request status was not updated
				updatedRequest := &checkpointrestorev1.CheckpointRequest{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: requestName, Namespace: namespace}, updatedRequest)).To(Succeed())
				Expect(updatedRequest.Status.Phase).To(Equal("Failed"))
			})
		})

		Describe("When the CheckpointRequest is in InProgress status", func() {
			It("should not update the status and requeue after 10 seconds", func() {
				checkpointRequest := checkpointrestorev1.CheckpointRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      requestName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointRequestSpec{
						PodReference: checkpointrestorev1.PodReference{
							Name:      podName,
							Namespace: namespace,
						},
						ContainerName: containerName,
					},
				}
				Expect(k8sClient.Create(ctx, &checkpointRequest)).To(Succeed())

				checkpointRequest.Status = checkpointrestorev1.CheckpointRequestStatus{
					Phase: "InProgress",
				}
				Expect(k8sClient.Status().Update(ctx, &checkpointRequest)).To(Succeed())
				result, err := controller.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      requestName,
						Namespace: namespace,
					},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(10 * time.Second))

				// Check that the request status was not updated
				updatedRequest := &checkpointrestorev1.CheckpointRequest{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: requestName, Namespace: namespace}, updatedRequest)).To(Succeed())
				Expect(updatedRequest.Status.Phase).To(Equal("InProgress"))
			})
		})

		Describe("When the CheckpointRequest is in Pending status", func() {
			It("should update the status to Completed and create a Checkpoint", func() {
				checkpointRequest := checkpointrestorev1.CheckpointRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      requestName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointRequestSpec{
						PodReference: checkpointrestorev1.PodReference{
							Name:      podName,
							Namespace: namespace,
						},
						ContainerName: containerName,
					},
				}
				Expect(k8sClient.Create(ctx, &checkpointRequest)).To(Succeed())

				result, err := controller.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      requestName,
						Namespace: namespace,
					},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeZero())

				// Check that the request status was updated
				updatedRequest := &checkpointrestorev1.CheckpointRequest{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: requestName, Namespace: namespace}, updatedRequest)).To(Succeed())
				Expect(updatedRequest.Status.Phase).To(Equal("Completed"))

				// Check that a Checkpoint was created
				checkpointList := &checkpointrestorev1.CheckpointList{}
				Expect(k8sClient.List(ctx, checkpointList, &client.ListOptions{
					Namespace: namespace,
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"checkpoint-request-name": requestName,
					}),
				})).To(Succeed())
				Expect(checkpointList.Items).To(HaveLen(1))
				Expect(checkpointList.Items[0].ObjectMeta.OwnerReferences[0].Name).To(Equal(requestName))
			})
		})

		Describe("When the Checkpoint Service fails", func() {
			BeforeEach(func() {
				checkpointService.mockedResultError = errors.New("mocked error")
			})

			It("should update the status to Failed", func() {
				checkpointRequest := checkpointrestorev1.CheckpointRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      requestName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointRequestSpec{
						PodReference: checkpointrestorev1.PodReference{
							Name:      podName,
							Namespace: namespace,
						},
						ContainerName: containerName,
					},
				}
				Expect(k8sClient.Create(ctx, &checkpointRequest)).To(Succeed())

				result, err := controller.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      requestName,
						Namespace: namespace,
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(result.RequeueAfter).To(BeZero())

				// Check that the request status was updated
				updatedRequest := &checkpointrestorev1.CheckpointRequest{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: requestName, Namespace: namespace}, updatedRequest)).To(Succeed())
				Expect(updatedRequest.Status.Phase).To(Equal("Failed"))

				// Check that a Checkpoint was not created
				checkpointList := &checkpointrestorev1.CheckpointList{}
				Expect(k8sClient.List(ctx, checkpointList, &client.ListOptions{
					Namespace: namespace,
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"checkpoint-request-name": requestName,
					}),
				})).To(Succeed())
				Expect(checkpointList.Items).To(BeEmpty())
			})
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})).To(Succeed())
		})
	})
})
