/*
Copyright The Kubernetes Authors.

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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	inventoryv1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta2"
	utiltesting "sigs.k8s.io/kueue/pkg/util/testing"
)

func TestClusterProfileReconcile(t *testing.T) {
	testNamespace := "kueue-system"
	cases := map[string]struct {
		clusters     []kueue.MultiKueueCluster
		cps          []inventoryv1alpha1.ClusterProfile
		reconcileFor string
		reconcileNS  string

		wantClusters []kueue.MultiKueueCluster
		wantError    error
	}{
		"new cluster profile creates multikueue cluster": {
			reconcileFor: "cp1",
			reconcileNS:  "cp-namespace",
			cps: []inventoryv1alpha1.ClusterProfile{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cp-namespace",
					},
				},
			},
			wantClusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
						Labels: map[string]string{
							ManagedByLabel: "kueue",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "inventory.x-k8s.io/v1alpha1",
								Kind:       "ClusterProfile",
								Name:       "cp1",
							},
						},
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
		},
		"changes in cluster profile updates multikueue cluster": {
			reconcileFor: "cp1",
			reconcileNS:  "cp-namespace",
			cps: []inventoryv1alpha1.ClusterProfile{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cp-namespace",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			clusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
						Labels: map[string]string{
							ManagedByLabel: "kueue",
						},
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
			wantClusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
						Labels: map[string]string{
							ManagedByLabel: "kueue",
							"foo":          "bar",
						},
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
		},
		"cluster profile deletion does not delete multikueue cluster": {
			reconcileFor: "cp1",
			reconcileNS:  "cp-namespace",
			cps: []inventoryv1alpha1.ClusterProfile{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cp1",
						Namespace:         "cp-namespace",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{"kueue.x-k8s.io/finalizer"},
					},
				},
			},
			clusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
						Labels: map[string]string{
							ManagedByLabel: "kueue",
						},
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
			wantClusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
						Labels: map[string]string{
							ManagedByLabel: "kueue",
						},
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
		},
		"should not update unmanaged multikueuecluster": {
			reconcileFor: "cp1",
			reconcileNS:  "cp-namespace",
			cps: []inventoryv1alpha1.ClusterProfile{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp1",
						Namespace: "cp-namespace",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			clusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
			wantClusters: []kueue.MultiKueueCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cp-namespace.cp1",
						Namespace: testNamespace,
					},
					Spec: kueue.MultiKueueClusterSpec{
						ClusterProfile: &kueue.ClusterProfile{
							Name:      "cp1",
							Namespace: "cp-namespace",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, _ := utiltesting.ContextWithLog(t)
			builder := getClientBuilder(ctx)

			builder = builder.WithLists(
				&kueue.MultiKueueClusterList{Items: tc.clusters},
				&inventoryv1alpha1.ClusterProfileList{Items: tc.cps},
			)

			for _, c := range tc.clusters {
				builder = builder.WithStatusSubresource(c.DeepCopy())
			}

			c := builder.Build()

			reconciler := newCPReconciler(c, testNamespace)

			_, gotErr := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: tc.reconcileFor, Namespace: tc.reconcileNS}})
			if diff := cmp.Diff(tc.wantError, gotErr); diff != "" {
				t.Errorf("unexpected error (-want/+got):\n%s", diff)
			}

			clusters := &kueue.MultiKueueClusterList{}
			listErr := c.List(ctx, clusters)

			if listErr != nil {
				t.Errorf("unexpected list clusters error: %s", listErr)
			}

			if diff := cmp.Diff(tc.wantClusters, clusters.Items, cmpopts.EquateEmpty(),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "OwnerReferences"),
				cmpopts.IgnoreTypes(metav1.TypeMeta{}),
				cmpopts.SortSlices(func(a, b metav1.Condition) bool { return a.Type < b.Type })); diff != "" {
				t.Errorf("unexpected clusters (-want/+got):\n%s", diff)
			}
		})
	}
}
