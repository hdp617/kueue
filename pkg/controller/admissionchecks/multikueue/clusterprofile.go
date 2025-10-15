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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	inventoryv1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta2"
)

const (
	// ManagedByLabel is a label added to MultiKueueCluster objects to indicate
	// that they are managed by this controller.
	ManagedByLabel = "kueue.x-k8s.io/managed-by"
)

// CPReconciler reconciles a ClusterProfile object
type CPReconciler struct {
	client       client.Client
	mkcNamespace string
}

func newCPReconciler(client client.Client, namespace string) *CPReconciler {
	return &CPReconciler{
		client:       client,
		mkcNamespace: namespace,
	}
}

//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=clusterprofiles,verbs=get;list;watch
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=multikueueclusters,verbs=get;list;watch;create;update;patch

func (r *CPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("Reconcile ClusterProfile")

	var cp inventoryv1alpha1.ClusterProfile
	if err := r.client.Get(ctx, req.NamespacedName, &cp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cp.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.createOrUpdateMultiKueueCluster(ctx, &cp)
}

func (r *CPReconciler) createOrUpdateMultiKueueCluster(ctx context.Context, cp *inventoryv1alpha1.ClusterProfile) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	var mkc kueue.MultiKueueCluster
	mkcName := fmt.Sprintf("%s.%s", cp.Namespace, cp.Name)
	err := r.client.Get(ctx, types.NamespacedName{Name: mkcName, Namespace: r.mkcNamespace}, &mkc)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	labels := cp.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[ManagedByLabel] = "kueue"

	if err != nil {
		mkc = kueue.MultiKueueCluster{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      mkcName,
				Namespace: r.mkcNamespace,
				Labels:    labels,
			},
			Spec: kueue.MultiKueueClusterSpec{
				ClusterProfile: &kueue.ClusterProfile{
					Name:      cp.Name,
					Namespace: cp.Namespace,
				},
			},
		}
		log.V(2).Info("Creating MultiKueueCluster", "MultiKueueCluster", klog.KObj(&mkc))
		return ctrl.Result{}, r.client.Create(ctx, &mkc)
	}

	if mkc.Labels[ManagedByLabel] != "kueue" {
		log.V(3).Info("MultiKueueCluster not managed by kueue, skipping update", "MultiKueueCluster", klog.KObj(&mkc))
		return ctrl.Result{}, nil
	}

	mkc.Labels = labels
	mkc.Spec.ClusterProfile = &kueue.ClusterProfile{
		Name:      cp.Name,
		Namespace: cp.Namespace,
	}
	log.V(2).Info("Updating MultiKueueCluster", "MultiKueueCluster", klog.KObj(&mkc))
	return ctrl.Result{}, r.client.Update(ctx, &mkc)
}

func (r *CPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&inventoryv1alpha1.ClusterProfile{}).
		Owns(&kueue.MultiKueueCluster{}).
		Complete(r)
}
