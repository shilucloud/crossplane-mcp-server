package main

import (
	"context"
	"fmt"
	"os"

	"github.com/shilucloud/crossplane-agent/tools"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("error creating dynamic client: %v\n", err)
		os.Exit(1)
	}

	clientset := kubernetes.NewForConfigOrDie(config)

	// List Operations, CronOperations, WatchOperations.
	summary, err := tools.ListOperations(context.Background(), dynamicClient)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Operations (%d) ===\n", len(summary.Operations))
	for _, op := range summary.Operations {
		fmt.Printf("  name: %s | phase: %s | duration: %s | message: %s\n",
			op.Name, op.Phase, op.Duration, op.Message)
	}

	fmt.Printf("\n=== CronOperations (%d) ===\n", len(summary.CronOperations))
	for _, op := range summary.CronOperations {
		fmt.Printf("  name: %s | schedule: %s | lastRun: %s | status: %s | totalRuns: %d\n",
			op.Name, op.Schedule, op.LastRunTime, op.LastRunStatus, op.TotalRuns)
	}

	fmt.Printf("\n=== WatchOperations (%d) ===\n", len(summary.WatchOperations))
	for _, op := range summary.WatchOperations {
		fmt.Printf("  name: %s | watching: %s/%s | lastTriggered: %s | triggers: %d | status: %s\n",
			op.Name, op.WatchingKind, op.WatchingName,
			op.LastTriggered, op.TriggerCount, op.Status)
	}

	// Get Crossplane Information.
	crossplaneVersion, err := tools.GetCrossplaneInfo(context.Background(), dynamicClient, clientset)
	if err != nil {
		fmt.Printf("error detecting crossplane version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n=== Crossplane Version ===\n")
	fmt.Printf("Version: %s | Major: %d | HasMRDs: %t | HasOperations: %t | HasNamespacedXRs: %t\n",
		crossplaneVersion.Version, crossplaneVersion.MajorVersion,
		crossplaneVersion.HasMRDs, crossplaneVersion.HasOperations, crossplaneVersion.HasNamespacedXRs)
	fmt.Printf("Total XRDs: %d | Namespaced XRDs: %d | Cluster XRDs: %d\n",
		crossplaneVersion.TotalXRDs, crossplaneVersion.NamespacedXRDs, crossplaneVersion.ClusterXRDs)

	fmt.Printf("Total number of providers: %s\n", crossplaneVersion.NumberOfProvider)
	fmt.Printf("Providers:\n")

	for _, p := range crossplaneVersion.Providers {
		fmt.Printf("  name: %s | version: %s | state: %s | installed: %t\n", p.Name, p.Version, p.State, p.Installed)
	}

	// List All XR's
	xrs, err := tools.ListXrs(context.Background(), dynamicClient)
	if err != nil {
		fmt.Println(err)
	}

	for _, xr := range xrs {
		fmt.Printf("Name: %s\n", xr.Name)

		fmt.Printf("Namespace: %s\n", xr.Namespace)
		fmt.Printf("Ready: %s\n", xr.Ready)
		fmt.Printf("Synced: %s\n", xr.Synced)
		fmt.Printf("compositionref: %s\n", xr.CompositionRef)
		fmt.Printf("age: %s\n", xr.Age)
		fmt.Printf("Group: %s\n", xr.Group)
		fmt.Printf("Scope: %s\n", xr.Scope)

		fmt.Println("----------------------------------------")
	}

	//mrs, err := tools.ListManagedResources(context.Background(), dynamicClient)
	//if err != nil {
	//	fmt.Println("error while listing mr")

	//	}

	//	for _, mr := range mrs {
	//		fmt.Printf("Name: %s\n", mr.Name)
	//
	//		fmt.Printf("age: %s\n", mr.Age)
	//		fmt.Printf("group: %s\n", mr.Group)
	//		fmt.Printf("kind: %s\n", mr.Kind)
	//		fmt.Printf("namespace: %s\n", mr.Namespace)
	//		fmt.Printf("ready: %s\n", mr.Ready)
	//		fmt.Printf("provider: %s\n", mr.Provider)
	//		fmt.Printf("synced: %s\n", mr.Synced)
	//	}

	mrDetails, err := tools.GetManagedResource(context.Background(), dynamicClient, "s3.aws.upbound.io", "v1beta2", "Bucket", "test-crossplane-connection-11222222", "default")

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("------mr detail------")

	fmt.Printf(mrDetails.Name)

}
