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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
)

var _ = Describe("CheckpointSchedule Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		checkpointschedule := &checkpointrestorev1.CheckpointSchedule{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind CheckpointSchedule")
			err := k8sClient.Get(ctx, typeNamespacedName, checkpointschedule)
			if err != nil && errors.IsNotFound(err) {
				resource := &checkpointrestorev1.CheckpointSchedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: checkpointrestorev1.CheckpointScheduleSpec{
						Schedule: "0 0 * * *",
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test-app"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &checkpointrestorev1.CheckpointSchedule{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance CheckpointSchedule")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the cron job was created")
			Expect(controllerReconciler.CronJobs).To(HaveLen(1))
		})

		It("should update the cron job when the schedule changes", func() {
			By("Reconciling the created resource")
			controllerReconciler := NewCheckpointScheduleReconciler(k8sClient, k8sClient.Scheme())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Updating the schedule")
			resource := &checkpointrestorev1.CheckpointSchedule{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			resource.Spec.Schedule = "5 * * * *"
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the cron job was updated")
			Expect(controllerReconciler.CronJobs).To(HaveLen(1))
			Expect(controllerReconciler.CronJobs[resourceName].Entries()).To(HaveLen(1))
		})
	})
})
