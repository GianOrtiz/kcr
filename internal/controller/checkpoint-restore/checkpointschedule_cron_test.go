package checkpointrestore

import (
	"context"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CheckpointSchedule CronJob", func() {
	Context("when the schedule of the cronjob is reached", func() {
		const resourceName = "test-resource"

		var (
			ctx                context.Context
			namespace          string
			typeNamespacedName types.NamespacedName
			checkpointSchedule *checkpointrestorev1.CheckpointSchedule
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespace = "ns-" + randStringRunes(5)
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})
		})

		Describe("when the schedule was deleted", func() {
			It("should do nothing", func() {
				controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())
				Expect(controllerReconciler.CronJob(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})).ToNot(Succeed())
			})
		})

		Describe("when the schedule exists", func() {
			BeforeEach(func() {
				checkpointSchedule = &checkpointrestorev1.CheckpointSchedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: checkpointrestorev1.CheckpointScheduleSpec{
						Schedule: "0 0 * * *",
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test-app"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, checkpointSchedule)).To(Succeed())
			})

			Describe("when there are no Pods referenced by the schedule selector", func() {
				It("should do nothing", func() {
					controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())
					Expect(controllerReconciler.CronJob(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})).ToNot(Succeed())
				})
			})

			Describe("when there are Pod referenced by the schedule selector", func() {
				BeforeEach(func() {
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: namespace,
							Labels: map[string]string{
								"app": "test-app",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
								},
							},
						},
					}
					Expect(k8sClient.Create(ctx, pod)).To(Succeed())
				})

				It("should create a new CheckpointRequest", func() {
					controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())
					Expect(controllerReconciler.CronJob(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})).To(Succeed())

					var checkpointRequests checkpointrestorev1.CheckpointRequestList
					Expect(k8sClient.List(ctx, &checkpointRequests, &client.ListOptions{
						Namespace: namespace,
					})).To(Succeed())
					Expect(checkpointRequests.Items).To(HaveLen(1))
					Expect(checkpointRequests.Items[0].Spec.PodReference.Name).To(Equal("test-pod"))
				})

				It("should update last run time", func() {
					controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())
					Expect(controllerReconciler.CronJob(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})).To(Succeed())

					var updatedCheckpointSchedule checkpointrestorev1.CheckpointSchedule
					Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedCheckpointSchedule)).To(Succeed())
					Expect(updatedCheckpointSchedule.Status.LastRunTime).ToNot(BeNil())
				})
			})
		})
	})
})
