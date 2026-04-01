package tools

import (
	"fmt"
	"os"

	"github.com/shilucloud/crossplane-agent/internal/logging"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	DynamicClient   dynamic.Interface
	Clientset       kubernetes.Interface
	DiscoveryClient discovery.DiscoveryInterface
	RestMapper      meta.RESTMapper
	ready           bool
)

func IsReady() bool {
	return ready
}

func InitClients() error {
	var config *rest.Config
	var err error

	logging.Info("initializing kubernetes clients")

	// try in-cluster first (when running as a pod)
	config, err = rest.InClusterConfig()
	if err != nil {
		logging.Info("not running in-cluster, trying kubeconfig")
		// fallback to kubeconfig for local dev
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("error building kubeconfig: %w", err)
		}
	}

	DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	logging.Info("dynamic client initialized")

	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	logging.Info("kubernetes clientset initialized")

	RestMapper, err = NewRESTMapper(config)
	if err != nil {
		return err
	}
	logging.Info("REST mapper initialized")

	DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	logging.Info("discovery client initialized")

	logging.Info("all kubernetes clients initialized successfully")
	ready = true
	return nil
}

func NewRESTMapper(kubeconfig *rest.Config) (meta.RESTMapper, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient)), nil
}
