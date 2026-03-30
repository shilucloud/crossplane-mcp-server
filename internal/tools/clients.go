package tools

import (
	"fmt"
	"os"

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
)

func InitClients() error {
	var config *rest.Config
	var err error

	// try in-cluster first (when running as a pod)
	config, err = rest.InClusterConfig()
	if err != nil {
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

	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	RestMapper, err = NewRESTMapper(config)
	if err != nil {
		return err
	}

	DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func NewRESTMapper(kubeconfig *rest.Config) (meta.RESTMapper, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient)), nil
}
