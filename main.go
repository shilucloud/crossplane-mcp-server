package main

import (
	"context"
	"fmt"
	"os"

	"github.com/shilucloud/crossplane-agent/tools"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {

	kubernetesClients, err := NewKubernetesClients("")
	if err != nil {
		fmt.Printf("error creating Kubernetes clients: %v\n", err)
		os.Exit(1)
	}

	dynamicClient := kubernetesClients.DynamicClient
	clientset := kubernetesClients.Clientset
	restMapper := kubernetesClients.RESTMapper
	discoveryClient := kubernetesClients.DiscoveryClient
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

	//mrDetails, err := tools.GetManagedResource(context.Background(), dynamicClient, "s3.aws.upbound.io", "v1beta2", "Bucket", "test-crossplane-connection-11222222", "default")

	//if err != nil {
	//	fmt.Println(err)
	//}

	//fmt.Println("------mr detail------")

	//fmt.Printf(mrDetails.Name)

	// list all providers
	providers, err := tools.ListProviders(context.Background(), dynamicClient)
	if err != nil {
		fmt.Printf("error listing providers: %v\n", err)
		os.Exit(1)
	}

	for _, p := range providers {
		fmt.Printf("Name: %s | Version: %s | State: %s | Installed: %t\n", p.Name, p.Version, p.State, p.Installed)
	}

	// check provider health
	providerHealth, err := tools.CheckAllProviderHealth(context.Background(), dynamicClient)
	if err != nil {
		fmt.Printf("error checking provider health: %v\n", err)
		os.Exit(1)
	}

	for _, ph := range providerHealth {
		fmt.Printf("Provider: %s | Healthy: %t\n", ph.ProviderName)
	}

	// Get events for a specific XR
	events, err := tools.GetEventsByUID(context.Background(), clientset, "df468c3f-f092-49ad-844d-453458ba594a")
	if err != nil {
		fmt.Printf("error getting events: %v\n", err)
	}

	fmt.Printf("\n=== Events for example-xr ===\n")
	for _, e := range events {
		fmt.Printf("LastSeen: %s | Type: %s | Reason: %s | Object: %s | Message: %s | Count: %d\n",
			e.LastSeen, e.Type, e.Reason, e.Object, e.Message, e.Count)
	}

	fmt.Println("==========================")

	// get condition for specific MR
	conditions, err := tools.GetConditions(context.Background(), dynamicClient, "s3.aws.upbound.io", "v1beta1", "buckets", "test-crossplane-connection-11222222", "")
	if err != nil {
		fmt.Printf("error getting conditions: %v\n", err)
	}

	fmt.Printf("\n=== Conditions for example-xr ===\n")
	for _, c := range conditions {
		fmt.Printf("Type: %s | Status: %s | Reason: %s | Message: %s | LastTransitionTime: %s | ObservedGeneration: %d\n",
			c.Type, c.Status, c.Reason, c.Message, c.LastTransitionTime, c.ObservedGeneration)
	}

	// get condition for specific
	xrconditions, err := tools.GetConditions(context.Background(), dynamicClient, "platform.example.com", "v1alpha1", "xbuckets", "my-bucket", "default")
	if err != nil {
		fmt.Printf("error getting conditions: %v\n", err)
	}

	fmt.Printf("\n=== Conditions for example-xr ===\n")
	for _, c := range xrconditions {
		fmt.Printf("Type: %s | Status: %s | Reason: %s | Message: %s | LastTransitionTime: %s | ObservedGeneration: %d\n",
			c.Type, c.Status, c.Reason, c.Message, c.LastTransitionTime, c.ObservedGeneration)
	}

	// get xrtree
	fmt.Println("Get xr tree for my-bucket")
	tools.GetXRTree(context.Background(), dynamicClient, "platform.example.com", "v1alpha1", "xbuckets", "my-bucket", "default")

	fmt.Println("\n>>> GetXRTree")
	tree, err := tools.GetXRTree(context.Background(), dynamicClient,
		"platform.example.com", "v1alpha1", "xnetworks", "my-network", "default")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  XR: %s/%s | ready: %s | synced: %s\n",
			tree.XRNamespace, tree.XRName, tree.XRReady, tree.XRSynced)
		fmt.Printf("  Composition: %s | mode: %s\n",
			tree.CompositionInfo.Name, tree.CompositionInfo.Mode)
		fmt.Println("  MRs:")
		for _, mr := range tree.MRs {
			fmt.Printf("    - %s/%s | ready: %s | synced: %s | providerConfig: %s\n",
				mr.Kind, mr.Name, mr.Ready, mr.Synced, mr.ProviderConfigName)
		}
	}

	fmt.Println("\n>>> DebugXR")
	debugResult, err := tools.DebugXR(
		context.Background(), dynamicClient, clientset,
		"platform.example.com", "v1alpha1", "xnetworks",
		"my-network", "default")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  XR: %s/%s | ready: %s | synced: %s\n",
			debugResult.XRNamespace, debugResult.XRName,
			debugResult.XRReady, debugResult.XRSynced)
		fmt.Printf("\n  === DIAGNOSIS ===\n")
		fmt.Printf("  Severity:     %s\n", debugResult.Diagnosis.Severity)
		fmt.Printf("  Root Cause:   %s\n", debugResult.Diagnosis.RootCause)
		fmt.Printf("  Affected Path:%s\n", debugResult.Diagnosis.AffectedPath)
		fmt.Printf("  Suggested Fix:%s\n", debugResult.Diagnosis.SuggestedFix)
		fmt.Printf("\n  === EVENTS ===\n")
		for _, d := range debugResult.Diagnosis.Details {
			fmt.Printf("  - %s\n", d)
		}
	}

	// list all the compositions:
	fmt.Println("-------printing all compositions--------")
	compositions, err := tools.ListComposition(context.Background(), dynamicClient)
	if err != nil {
		fmt.Errorf("Error while listing composition %s", err)
	}

	for _, composition := range compositions {
		fmt.Printf("Name: %s\n", composition.Name)
	}

	// discover all the gvr
	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		panic(err)
	}

	for _, list := range resources {
		gv, _ := schema.ParseGroupVersion(list.GroupVersion)

		for _, r := range list.APIResources {
			if r.Kind == "ProviderConfig" {
				fmt.Printf("Group: %s | Version: %s | Resource: %s\n",
					gv.Group,
					gv.Version,
					r.Name,
				)
			}
		}
	}

	fmt.Println("===================provider health==============")

	// get provider health summary
	providerHealthSummary, err := tools.GetProviderHealth(context.Background(), dynamicClient, &discoveryClient)
	if err != nil {
		fmt.Printf("error getting provider health summary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Provider Health Summary ===\n")
	fmt.Printf("Total Providers: %d\n", providerHealthSummary.TotalProviders)
	fmt.Printf("Healthy Providers: %d\n", providerHealthSummary.HealthyProviders)
	fmt.Printf("Unhealthy Providers: %d\n", providerHealthSummary.UnhealthyProviders)

	for _, ph := range providerHealthSummary.Providers {
		fmt.Println("========providers=============")
		fmt.Printf("Provider: %s | Healthy: %t\n", ph.ProviderName, ph.Healthy)
		for _, phconfig := range ph.ProviderConfigs {
			fmt.Printf("  - ProviderConfig: %s/%s | Ready: %s | Synced: %s | SecretRef: %s\n",
				phconfig.Namespace, phconfig.Name, phconfig.Ready, phconfig.Synced, phconfig.SecretRef)
		}
		for _, c := range ph.Conditions {
			fmt.Printf("  - Condition: Type: %s | Status: %s | Reason: %s | Message: %s | LastTransitionTime: %s | ObservedGeneration: %d\n",
				c.Type, c.Status, c.Reason, c.Message, c.LastTransitionTime, c.ObservedGeneration)
		}

		for _, phconfig := range ph.ProviderConfigs {
			fmt.Printf("  - ProviderConfig: %s/%s | Ready: %s | Synced: %s | SecretRef: %s\n",
				phconfig.Namespace, phconfig.Name, phconfig.Ready, phconfig.Synced, phconfig.SecretRef)
		}

	}

	fmt.Println("=======providerconfig==========")

	// check providerconfig
	providerConfig, err := tools.CheckProviderConfig(
		context.Background(),
		dynamicClient,
		clientset,
		restMapper,
		"ProviderConfig",
		"aws.m.upbound.io",
		"aws-provider",
		"default",
	)

	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("providerConfig Name %s\n", providerConfig.Name)
	fmt.Printf("providerConfig Namespace %s\n", providerConfig.CredentialNamespace)
	fmt.Printf("providerConfig secreet %s\n", providerConfig.SecretName)
	fmt.Printf("providerConfig Synced %s\n", providerConfig.Synced)
	fmt.Printf("providerConfig Conditions %s\n", providerConfig.Conditions)
	fmt.Printf("Credential exist %s\n", providerConfig.SecretExists)
	fmt.Printf("Providerconfig users %s\n", providerConfig.Users)

	fmt.Printf("======annotate resouce ====")
	fmt.Println("\n>>> AnnotateReconcile")
	reconcileResult, err := tools.AnnotateReconcile(
		context.Background(), dynamicClient,
		"platform.example.com", "v1alpha1", "xnetworks",
		"my-network", "default")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  triggered: %v | message: %s\n",
			reconcileResult.Triggered, reconcileResult.Message)
	}

	fmt.Println("\n>>> ExplainComposition")
	explanation, err := tools.ExplainComposition(context.Background(), dynamicClient, "xnetworks-aws")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  name: %s | mode: %s | forKind: %s\n",
			explanation.Name, explanation.Mode, explanation.ForKind)
		fmt.Printf("  totalResources: %d | totalFunctions: %d\n",
			explanation.TotalResources, explanation.TotalFunctions)
		fmt.Printf("\n  Summary:\n%s\n", explanation.Summary)
	}

	fmt.Println("\n>>> BuildDependencyGraph")
	graph, err := tools.BuildDependencyGraph(
		context.Background(), dynamicClient, clientset,
		"platform.example.com", "v1alpha1", "xnetworks",
		"my-network", "default")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Println(tools.PrintDependencyGraph(graph, "", true))
	}

	fmt.Println("\n>>> DebugMR")
	mrDebug, err := tools.DebugMR(
		context.Background(), dynamicClient, clientset, restMapper,
		"ec2.aws.m.upbound.io", "v1beta1", "Subnet",
		"my-network-3f0b34ccde33", "default")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  %s/%s | ready: %s | synced: %s\n",
			mrDebug.Kind, mrDebug.Name, mrDebug.Ready, mrDebug.Synced)
		fmt.Printf("  Severity: %s\n", mrDebug.Diagnosis.Severity)
		fmt.Printf("  RootCause: %s\n", mrDebug.Diagnosis.RootCause)
		fmt.Printf("  SuggestedFix: %s\n", mrDebug.Diagnosis.SuggestedFix)
	}

	fmt.Println("\n>>> DebugProvider")
	providerDebug, err := tools.DebugProvider(
		context.Background(), dynamicClient, clientset,
		"failure")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  provider: %s | healthy: %v | state: %s\n",
			providerDebug.ProviderName, providerDebug.Healthy, providerDebug.State)
		fmt.Printf("  Severity: %s\n", providerDebug.Diagnosis.Severity)
		fmt.Printf("  RootCause: %s\n", providerDebug.Diagnosis.RootCause)
		fmt.Printf("  AffectedMRs: %d\n", providerDebug.AffectedMRs)
	}

	fmt.Println("\n>>> DebugComposition")
	compDebug, err := tools.DebugComposition(
		context.Background(), dynamicClient,
		"failure")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Printf("  composition: %s | mode: %s | forKind: %s\n",
			compDebug.Name, compDebug.Mode, compDebug.ForKind)
		fmt.Printf("  Severity: %s\n", compDebug.Diagnosis.Severity)
		fmt.Printf("  RootCause: %s\n", compDebug.Diagnosis.RootCause)
		fmt.Printf("  XRs using it: %v\n", compDebug.XRsUsing)
		fmt.Printf("  Functions:\n")
		for _, f := range compDebug.Functions {
			fmt.Printf("    - %s | healthy: %v | installed: %v\n",
				f.Name, f.Healthy, f.Installed)
		}
	}

}
