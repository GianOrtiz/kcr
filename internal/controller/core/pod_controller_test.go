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

package core

import (
	"context"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	"github.com/GianOrtiz/kcr/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Pod Controller", func() {
	const (
		requestName    = "test-request"
		podName        = "test-pod"
		containerName  = "test-container"
		containerImage = "test-image"
		selectorKey    = "app"
		selectorValue  = "kcr-test"
	)

	Context("When reconciling a resource", func() {
		var (
			podController  *PodReconciler
			ctx            context.Context
			namespace      string
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespace = "ns-" + util.RandStringRunes(5)
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})).To(Succeed())

			// Create a test pod
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						selectorKey: selectorValue,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: containerImage,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			namespacedName = types.NamespacedName{
				Name:      podName,
				Namespace: namespace,
			}

			// Create our test controller
			podController = &PodReconciler{
				Client: k8sClient,
			}
		})

		Describe("When the Pod is not failed", func() {
			BeforeEach(func() {
				k8sClient.Status().Update(ctx, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				})
			})

			It("should not restore the Pod", func() {
				_, err := podController.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				var pod corev1.Pod
				k8sClient.Get(ctx, namespacedName, &pod)
				Expect(pod.Spec.Containers[0].Image).To(Equal(containerImage))
			})
		})

		Describe("When the Pod is failed", func() {
			BeforeEach(func() {
				k8sClient.Status().Update(ctx, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
					},
				})
			})

			Describe("When there is a latest checkpoint for the Pod", func() {
				selector := metav1.LabelSelector{
					MatchLabels: map[string]string{
						selectorKey: selectorValue,
					},
				}

				BeforeEach(func() {
					checkpoint := checkpointrestorev1.Checkpoint{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "test-checkpoint",
							Labels: map[string]string{
								"pod": podName,
							},
						},
						Spec: checkpointrestorev1.CheckpointSpec{
							Schedule: "* * * * *",
							Selector: &selector,
						},
					}
					Expect(k8sClient.Create(ctx, &checkpoint)).To(Succeed())
					checkpoint.Status.CheckpointImage = "kcr.io/checkpoint/test-checkpoint"
					checkpoint.Status.Phase = "ImageBuilt"
					Expect(k8sClient.Status().Update(ctx, &checkpoint)).To(Succeed())
				})

				It("should restore the Pod using the latest checkpoint image", func() {
					_, err := podController.Reconcile(ctx, reconcile.Request{
						NamespacedName: namespacedName,
					})
					Expect(err).NotTo(HaveOccurred())

					var pod corev1.Pod
					k8sClient.Get(ctx, namespacedName, &pod)
					Expect(pod.Spec.Containers[0].Image).To(Equal("kcr.io/checkpoint/test-checkpoint"))
				})
			})
		})
	})
})
