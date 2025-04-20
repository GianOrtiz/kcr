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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
)

var _ = Describe("Checkpoint Controller", func() {
	Context("When reconciling a resource", func() {
		const checkpointName = "test-checkpoint"

		var (
			ctx                context.Context
			namespace          string
			typeNamespacedName types.NamespacedName
			checkpoint         *checkpointrestorev1.Checkpoint
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespace = "ns-" + randStringRunes(5)
			typeNamespacedName = types.NamespacedName{
				Name:      checkpointName,
				Namespace: namespace,
			}

			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})).To(Succeed())

			now := metav1.Now()
			checkpoint = &checkpointrestorev1.Checkpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      checkpointName,
					Namespace: namespace,
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
			Expect(k8sClient.Create(ctx, checkpoint)).To(Succeed())
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})
		})

		Describe("when the checkpoint phase is ImageBuilt", func() {
			BeforeEach(func() {
				checkpoint.Status.Phase = "ImageBuilt"
				Expect(k8sClient.Status().Update(ctx, checkpoint)).To(Succeed())
			})

			It("should not change the phase and ignore the reconcile loop", func() {
				imageBuilder := mockImageBuilder{mockedResult: nil}
				controllerReconciler := &CheckpointReconciler{
					Client:       k8sClient,
					Scheme:       k8sClient.Scheme(),
					ImageBuilder: &imageBuilder,
				}

				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})
		})

		Describe("when the checkpoint phase is Failed", func() {
			BeforeEach(func() {
				checkpoint.Status.Phase = "Failed"
				Expect(k8sClient.Status().Update(ctx, checkpoint)).To(Succeed())
			})

			It("should not change the phase and ignore the reconcile loop", func() {
				imageBuilder := mockImageBuilder{mockedResult: nil}
				controllerReconciler := &CheckpointReconciler{
					Client:       k8sClient,
					Scheme:       k8sClient.Scheme(),
					ImageBuilder: &imageBuilder,
				}

				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				var updatedCheckpoint checkpointrestorev1.Checkpoint
				Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedCheckpoint)).To(Succeed())
				Expect(updatedCheckpoint.Status.Phase).To(Equal("Failed"))

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})
		})

		Describe("when the checkpoint phase is Processing", func() {
			BeforeEach(func() {
				checkpoint.Status.Phase = "Processing"
				Expect(k8sClient.Status().Update(ctx, checkpoint)).To(Succeed())
			})

			It("should not change the phase and requeue for later", func() {
				imageBuilder := mockImageBuilder{mockedResult: nil}
				controllerReconciler := &CheckpointReconciler{
					Client:       k8sClient,
					Scheme:       k8sClient.Scheme(),
					ImageBuilder: &imageBuilder,
				}

				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				var updatedCheckpoint checkpointrestorev1.Checkpoint
				Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedCheckpoint)).To(Succeed())
				Expect(updatedCheckpoint.Status.Phase).To(Equal("Processing"))

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeTrue())
			})
		})

		Describe("when the checkpoint phase is Created", func() {
			BeforeEach(func() {
				checkpoint.Status.Phase = "Created"
				Expect(k8sClient.Status().Update(ctx, checkpoint)).To(Succeed())
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
})
