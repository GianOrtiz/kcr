package controller

import (
	"context"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Deployment Controller", func() {
	const (
		deploymentName      = "test-deployment"
		deploymentNamespace = "default"
		schedule            = "* * * * *"
	)

	Context("When reconciling a resource", func() {
		Context("and the deployment has "+CHECKPOINT_RESTORE_SCHEDULE_ANNOTATION+" annotation", func() {
			Context("and the annotation follows a cronjob schedule", func() {
				It("should create a new CheckpointSchedule for the deployment with the given schedule", func() {
					selectorKey := "app"
					selectorValue := "kcr-test"
					selector := metav1.LabelSelector{
						MatchLabels: map[string]string{
							selectorKey: selectorValue,
						},
					}
					deploymentName := "kcr-test"
					deploymentNamespace := "default"
					ctx := context.Background()
					deployment := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: deploymentNamespace,
							Name:      deploymentName,
							Annotations: map[string]string{
								CHECKPOINT_RESTORE_SCHEDULE_ANNOTATION: schedule,
							},
						},
						Spec: appsv1.DeploymentSpec{
							Selector: &selector,
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										selectorKey: selectorValue,
									},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "kcr-test",
											Image: "busybox",
											Ports: []corev1.ContainerPort{
												{
													ContainerPort: 80,
												},
											},
										},
									},
								},
							},
						},
					}
					Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

					namespacedName := types.NamespacedName{
						Name:      deploymentName,
						Namespace: deploymentNamespace,
					}
					deploymentReconciler := DeploymentReconciler{
						Client: k8sClient,
						Scheme: k8sClient.Scheme(),
					}
					_, err := deploymentReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
					Expect(err).NotTo(HaveOccurred())

					var checkpointSchedule checkpointrestorev1.CheckpointSchedule
					err = k8sClient.Get(ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}, &checkpointSchedule)
					Expect(err).ToNot(HaveOccurred())
					Expect(checkpointSchedule.Spec.Schedule).To(Equal(schedule))
				})
			})
		})
	})
})
