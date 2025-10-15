/*
Copyright 2024 The Kubernetes Authors.

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

package multikueue

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inventoryv1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	"sigs.k8s.io/kueue/pkg/util/testing"
	utiltesting "sigs.k8s.io/kueue/pkg/util/testing"
	"sigs.k8s.io/kueue/test/util"
)

var _ = ginkgo.Describe("MultiKueue admission check with cluster profile", func() {
	var (
		managerNs *corev1.Namespace
		workerNs  *corev1.Namespace

		ac   *kueue.AdmissionCheck
		cq   *kueue.ClusterQueue
		lq   *kueue.LocalQueue
		wl   *kueue.Workload
		prof *inventoryv1alpha1.ClusterProfile
	)

	ginkgo.BeforeEach(func() {
		managerNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "multikueue-",
			},
		}
		gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, managerNs)).To(gomega.Succeed())

		workerNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: managerNs.Name,
			},
		}
		gomega.Expect(worker1TestCluster.client.Create(worker1TestCluster.ctx, workerNs)).To(gomega.Succeed())

		ac = testing.MakeAdmissionCheck("ac").
			ControllerName(kueue.MultiKueueControllerName).
			Parameters(kueue.GroupVersion.Group, "MultiKueueConfig", managerMultiKueueConfig.Name).
			Obj()
		gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, ac)).To(gomega.Succeed())

		cq = testing.MakeClusterQueue("cq").
			AdmissionChecks(kueue.AdmissionCheckReference(ac.Name)).
			Obj()
		gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, cq)).To(gomega.Succeed())

		lq = testing.MakeLocalQueue("lq", managerNs.Name).ClusterQueue(cq.Name).Obj()
		gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, lq)).To(gomega.Succeed())

		wl = testing.MakeWorkload("wl", managerNs.Name).Queue(kueue.LocalQueueName(lq.Name)).Obj()
		gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, wl)).To(gomega.Succeed())

		prof = &inventoryv1alpha1.ClusterProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prof",
				Namespace: managersConfigNamespace.Name,
			},
		}
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(util.DeleteNamespace(managerTestCluster.ctx, managerTestCluster.client, managerNs)).To(gomega.Succeed())
		gomega.Expect(util.DeleteNamespace(worker1TestCluster.ctx, worker1TestCluster.client, workerNs)).To(gomega.Succeed())
		util.ExpectObjectToBeDeleted(managerTestCluster.ctx, managerTestCluster.client, cq, true)
		util.ExpectObjectToBeDeleted(managerTestCluster.ctx, managerTestCluster.client, ac, true)
	})

	ginkgo.It("Should admit a workload if the clusterProfile is valid", func() {
		ginkgo.By("Create a cluster profile", func() {
			prof.Spec.ClusterManager.Name = "test-manager"
			gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, prof)).To(gomega.Succeed())
			ginkgo.DeferCleanup(managerTestCluster.client.Delete, managerTestCluster.ctx, prof)
		})

		ginkgo.By("Create a multikueue cluster", func() {
			cluster := testing.MakeMultiKueueCluster("worker1").
				ClusterProfile(prof.Name, prof.Namespace).
				Obj()
			gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, cluster)).To(gomega.Succeed())
			ginkgo.DeferCleanup(managerTestCluster.client.Delete, managerTestCluster.ctx, cluster)
		})

		ginkgo.By("Wait for the workload to be admitted", func() {
			gomega.Eventually(func(g gomega.Gomega) {
				var updatedWl kueue.Workload
				g.Expect(managerTestCluster.client.Get(managerTestCluster.ctx, client.ObjectKeyFromObject(wl), &updatedWl)).To(gomega.Succeed())
				g.Expect(updatedWl.Status.Conditions).To(utiltesting.HaveCondition(kueue.WorkloadAdmitted))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})

	ginkgo.It("Should not admit a workload if the clusterProfile is not found", func() {
		ginkgo.By("Create a multikueue cluster", func() {
			cluster := testing.MakeMultiKueueCluster("worker1").
				ClusterProfile(prof.Name, prof.Namespace).
				Obj()
			gomega.Expect(managerTestCluster.client.Create(managerTestCluster.ctx, cluster)).To(gomega.Succeed())
			ginkgo.DeferCleanup(managerTestCluster.client.Delete, managerTestCluster.ctx, cluster)
		})

		ginkgo.By("Check the workload is not admitted", func() {
			gomega.Consistently(func(g gomega.Gomega) {
				var updatedWl kueue.Workload
				g.Expect(managerTestCluster.client.Get(managerTestCluster.ctx, client.ObjectKeyFromObject(wl), &updatedWl)).To(gomega.Succeed())
				g.Expect(updatedWl.Status.Conditions).NotTo(utiltesting.HaveCondition(kueue.WorkloadAdmitted))
			}, util.Timeout, util.Interval).Should(gomega.Succeed())
		})
	})
})
