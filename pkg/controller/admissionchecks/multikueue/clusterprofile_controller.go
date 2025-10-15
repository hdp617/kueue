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
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta2"
)

const (
	// ManagedByLabel is a label added to MultiKueueCluster objects to indicate
	// that they are managed by this controller.
	ManagedByLabel = "kueue.x-k8s.io/managed-by"
)

// ClusterProfileReconciler reconciles a ClusterProfile object
type ClusterProfileReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewClusterProfileReconciler(client client.Client, scheme *runtime.Scheme) *ClusterProfileReconciler {
	return &ClusterProfileReconciler{
		client: client,
		scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=clusterprofiles,verbs=get;list;watch
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=multikueueclusters,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("Reconciling ClusterProfile")

	var cp inventoryv1alpha1.ClusterProfile
	if err := r.client.Get(ctx, req.NamespacedName, &cp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !cp.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, &cp)
	}

	return r.handleUpdate(ctx, &cp)
}

func (r *ClusterProfileReconciler) handleUpdate(ctx context.Context, cp *inventoryv1alpha1.ClusterProfile) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	var mkc kueue.MultiKueueCluster
	err := r.client.Get(ctx, types.NamespacedName{Name: cp.Name}, &mkc)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	if err != nil {
		// Not found, create a new one.
		mkc = kueue.MultiKueueCluster{
			ObjectMeta: ctrl.ObjectMeta{
				Name: cp.Name,
				Labels: map[string]string{
					ManagedByLabel: "kueue",
				},
			},
			Spec: kueue.MultiKueueClusterSpec{
				ClusterProfile: kueue.ClusterProfile{
					Name:      cp.Name,
					Namespace: cp.Namespace,
				},
			},
		}
		log.V(2).Info("Creating MultiKueueCluster", "MultiKueueCluster", klog.KObj(&mkc))
		return ctrl.Result{}, r.client.Create(ctx, &mkc)
	}

	// Found, update it.
	mkc.Spec.ClusterProfile.Name = cp.Name
	mkc.Spec.ClusterProfile.Namespace = cp.Namespace
	log.V(2).Info("Updating MultiKueueCluster", "MultiKueueCluster", klog.KObj(&mkc))
	return ctrl.Result{}, r.client.Update(ctx, &mkc)
}

func (r *ClusterProfileReconciler) handleDelete(ctx context.Context, cp *inventoryv1alpha1.ClusterProfile) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	var mkc kueue.MultiKueueCluster
	if err := r.client.Get(ctx, types.NamespacedName{Name: cp.Name}, &mkc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if val, ok := mkc.Labels[ManagedByLabel]; !ok || val != "kueue" {
		// Not managed by us, do nothing.
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Deleting MultiKueueCluster", "MultiKueueCluster", klog.KObj(&mkc))
	return ctrl.Result{}, r.client.Delete(ctx, &mkc)
}

func (r *ClusterProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&inventoryv1alpha1.ClusterProfile{}).
		Owns(&kueue.MultiKueueCluster{}).
		Complete(r)
}
