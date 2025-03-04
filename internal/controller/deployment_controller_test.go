package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				It("should add the deployment to monitor", func() {
					selectorKey := "app"
					selectorValue := "kcr-test"
					selector := metav1.LabelSelector{
						MatchLabels: map[string]string{
							selectorKey: selectorValue,
						},
					}
					ctx := context.Background()
					deployment := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "kcr-test",
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
					fmt.Println("ESTOU AQUI")
					Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
					Eventually(func(g Gomega) {
						isMonitoringDeployment := false
						for _, monitoredSelectors := range deploymentReconciler.MonitoredDeploymentSelectors {
							for key, value := range monitoredSelectors.MatchLabels {
								if key == selectorKey && value == selectorValue {
									isMonitoringDeployment = true
									break
								}
							}
						}
						g.Expect(isMonitoringDeployment).To(Equal(true))
					}, 10, 1)

				})
			})
		})
	})
})
