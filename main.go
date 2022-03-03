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

package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"git.company.tld/platform/operator-envconfig/pkg/clientcache"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"git.company.tld/platform/operator-envconfig/api/v1alpha1"
	"git.company.tld/platform/operator-envconfig/controllers"
	//envapi "git.company.tld/platform/operator-environment/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//utilruntime.Must(envapi.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func logEncodeTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var namespace string
	var tokensFile string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "", "Limit operator service to 1 namespace for devel purpose.")
	flag.StringVar(&tokensFile, "tokens-file", "/var/run/tokens-config/tokens-data", "Where admin-tokens file located. With json config [{'api_url': 'x', 'api_token': 'x'}, ]")

	logOpts := zap.Options{
		Development: true,
	}
	logOpts.EncoderConfigOptions = append(logOpts.EncoderConfigOptions, func(ec *zapcore.EncoderConfig) { ec.EncodeTime = logEncodeTime })
	logOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logOpts)))

	if len(namespace) == 0 {
		namespace = getWatchNamespace()
	}
	if len(namespace) > 0 {
		setupLog.Info("Watch namespace is set", "namespace", namespace)
	}

	syncPeriod := time.Hour
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Namespace:              namespace,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e3db711a.company.tld",
		SyncPeriod:             &syncPeriod,
		//NewCache: cache.BuilderWithOptions(cache.Options{
		//	SelectorsByObject: addProjectSelector(nil),
		//}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	uncachedClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to fetch config connection")
		os.Exit(1)
	}
	//ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	//reconcilerConfig := config.InitConfig()

	// Fetch ClientConfigs and create the clientCache
	clientCache := clientcache.New(mgr.GetClient(), uncachedClient, scheme)

	allAdminClientConfigs, err := clientCache.GetRemoteRestConfigsFromFile(tokensFile)
	if err != nil {
		setupLog.Error(err, "unable to get cluster connections configs")
		os.Exit(1)
	}

	additionalClusters := make([]cluster.Cluster, 0)
	for _, singleRestConfig := range *allAdminClientConfigs {
		// Add cluster for each remote kubernetes to the manager
		var singleCluster cluster.Cluster
		singleCluster, err = cluster.New(&singleRestConfig, func(o *cluster.Options) {
			o.Scheme = scheme
			o.NewCache = cache.BuilderWithOptions(cache.Options{SelectorsByObject: addProjectSelector(nil)})
		})
		if err != nil {
			setupLog.Error(err, "unable to create manager cluster connection", "cluster", singleRestConfig.Host)
			os.Exit(1)
		}
		err = mgr.Add(singleCluster)
		if err != nil {
			setupLog.Error(err, "unable to add cluster to manager")
			os.Exit(1)
		}

		fmt.Printf("Add to cache client config host %s cluster host %s client %v CACHE %v\n", singleRestConfig.Host, singleCluster.GetConfig().Host, singleCluster.GetClient(), singleCluster.GetCache())
		clientCache.AddClient(singleRestConfig.Host, singleCluster.GetClient())
		if smClient, err := clientCache.GetRemoteClient(singleRestConfig.Host); err == nil {
			fmt.Printf("Got from cache host %s client %v\n", singleRestConfig.Host, smClient)
		}
		additionalClusters = append(additionalClusters, singleCluster)
	}

	if err = (&controllers.EnvconfigReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		ClientCache:    clientCache,
	}).SetupWithManager(mgr, additionalClusters, namespace); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Envconfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func addProjectSelector(selectors cache.SelectorsByObject) cache.SelectorsByObject {
	ns := &corev1.Namespace{}
	secret := &corev1.Secret{}
	selector := labels.NewSelector()
	projectLabelRequirement, _ := labels.NewRequirement("mylabel", selection.Exists, []string{})
	selector = selector.Add(*projectLabelRequirement)
	newSelectors := cache.SelectorsByObject{
		secret: {Label: selector},
		ns:     {Label: selector},
	}
	if selectors == nil {
		return newSelectors
	}
	selectors[ns] = newSelectors[ns]
	selectors[secret] = newSelectors[secret]
	return selectors
}
