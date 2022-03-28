/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"git.company.tld/platform/operator-envconfig/pkg/clientcache"
	"git.company.tld/platform/operator-envconfig/api/v1alpha1"
)

// EnvconfigReconciler reconciles a Envconfig object
type EnvconfigReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ClientCache    *clientcache.ClientCache
}

//+kubebuilder:rbac:groups=company.tld,resources=envconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=company.tld,resources=envconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=company.tld,resources=envconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *EnvconfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	myLog := log.FromContext(ctx).WithValues("envconfig", req.NamespacedName)

	for apiUrl, cl := range r.ClientCache.GetRemoteClients() {
		fmt.Printf("api %s client %v\n", apiUrl, cl)
		var nsl corev1.NamespaceList
		if err := cl.List(ctx, &nsl); err == nil {
			// so here next step all namespaces from all clusters will be printed, like if they lived in apiUrl cluster
			for _, nsI := range nsl.Items {
				fmt.Println("Found ns", nsI.Name, "in cluster ", apiUrl)
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnvconfigReconciler) SetupWithManager(mgr ctrl.Manager, clusters []*cluster.Cluster) error {
	mainController := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Envconfig{})

	ecLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		labels := mapObj.GetLabels()
		ecName := labels["mylabel"]
		ecProject := labels["anotherlabel"]

		if ecName != "" && ecProject != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ecProject, Name: ecName}})
		}
		return requests
	}

	mainController = mainController.Watches(&source.Kind{Type: &corev1.Namespace{}}, handler.EnqueueRequestsFromMapFunc(ecLabelFilter))

	for _, cP := range clusters {
                cluster := *cP
		mainController = mainController.Watches(source.NewKindWithCache(&corev1.Namespace{}, cluster.GetCache()),
			handler.EnqueueRequestsFromMapFunc(ecLabelFilter))
	}

	return mainController.Complete(r)
}
