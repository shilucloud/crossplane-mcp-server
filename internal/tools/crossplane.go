package tools

import (
	"context"

	crossplane "github.com/shilucloud/crossplane-mcp-server/tools"
)

func init() {
	// list_xrs
	registerTool("list_xrs",
		"List all Composite Resources (XRs) across all XRDs in the cluster. Returns name, namespace, kind, ready/synced status, composition ref, scope, and group.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListXrs(ctx, DynamicClient)
		},
	)

	// list_xrds
	registerTool("list_xrds",
		"List all CompositeResourceDefinitions (XRDs) in the cluster. Returns name, kind, group, version, scope, established status, XR instance count, and associated compositions.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListXRDs(ctx, DynamicClient)
		},
	)

	// list_managed_resources
	registerTool("list_managed_resources",
		"List all Managed Resources (MRs) across all providers in the cluster. Only lists CRDs owned by a ProviderRevision with at least one instance.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListManagedResources(ctx, DynamicClient)
		},
	)

	// get_managed_resource
	type getMRParams struct {
		Group     string `json:"group"     jsonschema:"API group e.g. s3.aws.upbound.io"`
		Version   string `json:"version"   jsonschema:"API version e.g. v1beta1"`
		Kind      string `json:"kind"      jsonschema:"Kind e.g. Bucket"`
		Name      string `json:"name"      jsonschema:"Name of the managed resource"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("get_managed_resource",
		"Deep inspect a single Managed Resource by group, kind, and name. Returns spec, status, conditions, annotations, and the XR that owns it.",
		func(ctx context.Context, p getMRParams) (any, error) {
			return crossplane.GetManagedResource(ctx, DynamicClient, p.Group, p.Version, p.Kind, p.Name, p.Namespace)
		},
	)

	// get_xr_tree
	type xrParams struct {
		Group     string `json:"group"     jsonschema:"XR API group e.g. platform.example.com"`
		Version   string `json:"version"   jsonschema:"XR version e.g. v1alpha1"`
		Resource  string `json:"resource"  jsonschema:"XR plural resource name e.g. xnetworks"`
		Name      string `json:"name"      jsonschema:"Name of the XR"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("get_xr_tree",
		"Get the full resource tree for a Crossplane XR: XR → Composition (with pipeline) → Managed Resources → ProviderConfig details.",
		func(ctx context.Context, p xrParams) (any, error) {
			return crossplane.GetXRTree(ctx, DynamicClient, p.Group, p.Version, p.Resource, p.Name, p.Namespace)
		},
	)

	// get_conditions
	type condParams struct {
		Group     string `json:"group"     jsonschema:"API group e.g. s3.aws.upbound.io"`
		Version   string `json:"version"   jsonschema:"API version e.g. v1beta1"`
		Resource  string `json:"resource"  jsonschema:"Plural resource name e.g. buckets"`
		Name      string `json:"name"      jsonschema:"Name of the resource"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("get_conditions",
		"Get status conditions for any Crossplane resource (XR, MR, ProviderConfig). Returns type, status, reason, message, lastTransitionTime, and observedGeneration.",
		func(ctx context.Context, p condParams) (any, error) {
			return crossplane.GetConditions(ctx, DynamicClient, p.Group, p.Version, p.Resource, p.Name, p.Namespace)
		},
	)

	// get_crossplane_info
	registerTool("get_crossplane_info",
		"Get Crossplane installation info: version, feature flags (MRDs, Operations, NamespacedXRs), XRD counts, and installed providers with health state.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.GetCrossplaneInfo(ctx, DynamicClient, Clientset)
		},
	)

	// list_providers
	registerTool("list_providers",
		"List all Crossplane providers installed in the cluster. Returns name, version, health state, and installed status.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListProviders(ctx, DynamicClient)
		},
	)

	// get_provider_health
	registerTool("get_provider_health",
		"Get a full health summary for all Crossplane providers: healthy/installed status, conditions, ProviderConfigs with secretRef, and MR counts.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.GetProviderHealth(ctx, DynamicClient, DiscoveryClient)
		},
	)

	// check_provider_config
	type pcParams struct {
		Kind      string `json:"kind"      jsonschema:"Kind e.g. ProviderConfig"`
		Group     string `json:"group"     jsonschema:"API group e.g. aws.upbound.io"`
		Name      string `json:"name"      jsonschema:"Name of the ProviderConfig"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("check_provider_config",
		"Inspect a ProviderConfig: credential type, secret name/namespace, whether the secret exists, ready/synced status, user count, and conditions.",
		func(ctx context.Context, p pcParams) (any, error) {
			return crossplane.CheckProviderConfig(ctx, DynamicClient, Clientset, RestMapper, p.Kind, p.Group, p.Name, p.Namespace)
		},
	)

	// annotate_reconcile
	type reconcileParams struct {
		Group     string `json:"group"     jsonschema:"API group e.g. platform.example.com"`
		Version   string `json:"version"   jsonschema:"API version e.g. v1alpha1"`
		Resource  string `json:"resource"  jsonschema:"Plural resource name e.g. xnetworks"`
		Name      string `json:"name"      jsonschema:"Name of the resource"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("annotate_reconcile",
		"Trigger an immediate reconcile on a Crossplane XR or MR by patching the crossplane.io/paused=false and reconcile.crossplane.io/last-triggered annotations.",
		func(ctx context.Context, p reconcileParams) (any, error) {
			return crossplane.AnnotateReconcile(ctx, DynamicClient, p.Group, p.Version, p.Resource, p.Name, p.Namespace)
		},
	)

	// list_compositions
	registerTool("list_compositions",
		"List all Compositions in the cluster. Returns name, mode, and pipeline steps for each Composition.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListComposition(ctx, DynamicClient)
		},
	)

	// explain_composition
	registerTool("explain_composition",
		"Deep explain a Composition: mode, forKind, pipeline steps, functions, resources created, default field values, and all field patches (from/to paths, transforms, combines).",
		func(ctx context.Context, p struct {
			Name string `json:"name" jsonschema:"Name of the Composition"`
		}) (any, error) {
			return crossplane.ExplainComposition(ctx, DynamicClient, p.Name)
		},
	)

	// debug_composition
	registerTool("debug_composition",
		"Debug a Crossplane Composition. Checks pipeline functions (installed/healthy), finds XRs using it, validates mode, and returns a structured diagnosis with severity, root cause, and suggested fix.",
		func(ctx context.Context, p struct {
			Name string `json:"name" jsonschema:"Name of the Composition"`
		}) (any, error) {
			return crossplane.DebugComposition(ctx, DynamicClient, p.Name)
		},
	)

	// debug_mr
	type debugMRParams struct {
		Group     string `json:"group"     jsonschema:"API group e.g. ec2.aws.upbound.io"`
		Version   string `json:"version"   jsonschema:"API version e.g. v1beta1"`
		Kind      string `json:"kind"      jsonschema:"Kind e.g. Subnet"`
		Name      string `json:"name"      jsonschema:"Name of the managed resource"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("debug_mr",
		"Debug a Crossplane Managed Resource. Inspects conditions, events, and ProviderConfig credentials. Returns a structured diagnosis: missing secret, access denied, quota exceeded, etc.",
		func(ctx context.Context, p debugMRParams) (any, error) {
			return crossplane.DebugMR(ctx, DynamicClient, Clientset, RestMapper, p.Group, p.Version, p.Kind, p.Name, p.Namespace)
		},
	)

	// debug_provider
	registerTool("debug_provider",
		"Debug a Crossplane provider. Checks installed/healthy status, conditions, events, and failing MR counts. Diagnoses dependency conflicts, image pull errors, and credential issues.",
		func(ctx context.Context, p struct {
			Name string `json:"name" jsonschema:"Name of the provider e.g. provider-aws"`
		}) (any, error) {
			return crossplane.DebugProvider(ctx, DynamicClient, Clientset, p.Name)
		},
	)

	// build_dependency_graph
	type graphParams struct {
		Group     string `json:"group"     jsonschema:"XR API group e.g. platform.example.com"`
		Version   string `json:"version"   jsonschema:"XR version e.g. v1alpha1"`
		Resource  string `json:"resource"  jsonschema:"XR plural resource name e.g. xnetworks"`
		Name      string `json:"name"      jsonschema:"Name of the XR"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("build_dependency_graph",
		"Build a full dependency graph for a Crossplane XR: XR → Composition → Functions → Managed Resources → ProviderConfigs → Secrets, with health status at each node.",
		func(ctx context.Context, p graphParams) (any, error) {
			graph, err := crossplane.BuildDependencyGraph(ctx, DynamicClient, Clientset, p.Group, p.Version, p.Resource, p.Name, p.Namespace)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"graph": graph,
				"tree":  crossplane.PrintDependencyGraph(graph, "", true),
			}, nil
		},
	)

	// debug_xr
	type debugXRParams struct {
		Group     string `json:"group"     jsonschema:"XR API group e.g. platform.example.com"`
		Version   string `json:"version"   jsonschema:"XR version e.g. v1alpha1"`
		Resource  string `json:"resource"  jsonschema:"XR plural resource name e.g. xnetworks"`
		Name      string `json:"name"      jsonschema:"Name of the XR"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}

	registerTool("debug_xr",
		"Deep debug a Crossplane XR (Composite Resource). Builds the full XR tree (XR → Composition → Managed Resources → Providers), collects events, and returns a structured diagnosis with root cause, severity, and suggested fixes. Detects issues like missing compositions, function failures, provider errors, credential issues, and MR sync failures.",
		func(ctx context.Context, p debugXRParams) (any, error) {
			return crossplane.DebugXR(
				ctx,
				DynamicClient,
				Clientset,
				p.Group,
				p.Version,
				p.Resource,
				p.Name,
				p.Namespace,
			)
		},
	)

	// describe_xrd
	registerTool("describe_xrd",
		"Describe a CompositeResourceDefinition (XRD). Returns kind, group, version, scope, established status, and all spec.parameters fields with types, enums, defaults, and descriptions.",
		func(ctx context.Context, p struct {
			Name string `json:"name" jsonschema:"Name of the XRD e.g. xnetworks.platform.example.com"`
		}) (any, error) {
			return crossplane.DescribeXRD(ctx, DynamicClient, p.Name)
		},
	)

	// validate_xr
	type validateParams struct {
		Group     string `json:"group"     jsonschema:"XR API group e.g. platform.example.com"`
		Version   string `json:"version"   jsonschema:"XR version e.g. v1alpha1"`
		Resource  string `json:"resource"  jsonschema:"XR plural resource name e.g. xnetworks"`
		Name      string `json:"name"      jsonschema:"Name of the XR"`
		Namespace string `json:"namespace" jsonschema:"Namespace (empty for cluster-scoped)"`
	}
	registerTool("validate_xr",
		"Validate a Crossplane XR against its XRD schema and Composition. Checks missing required fields, enum violations, type mismatches, invalid AWS regions, and value overrides vs composition defaults.",
		func(ctx context.Context, p validateParams) (any, error) {
			return crossplane.ValidateXR(ctx, DynamicClient, p.Group, p.Version, p.Resource, p.Name, p.Namespace)
		},
	)

	// list_operations
	registerTool("list_operations",
		"List all Crossplane Operations, CronOperations, and WatchOperations (Crossplane v2 only). Returns phase/timing for one-off ops, schedule/run history for cron ops, and trigger info for watch ops.",
		func(ctx context.Context, _ struct{}) (any, error) {
			return crossplane.ListOperations(ctx, DynamicClient)
		},
	)
}
