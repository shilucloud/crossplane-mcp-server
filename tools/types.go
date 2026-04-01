package tools

import "time"

// used in get_xr_tree and composition tool
type ProviderConfigInfo struct {
	Name   string
	Group  string
	Ready  string
	Synced string
	Kind   string
}

type ResourceField struct {
	Name   string
	Fields map[any]any
}

type ResourceProviderInfo struct {
	ApiVersion         string
	Kind               string
	ProviderConfigInfo ProviderConfigInfo
}

type ResourceInfo struct {
	Name string

	ResourceProvider ResourceProviderInfo
}

// used in composition tool (list)
type PipelineResourceInfo struct {
	Name string
	Kind string
}

// used in composition tool (list)
type PipelineInfo struct {
	StepName     string
	FunctionName string
	Resources    []PipelineResourceInfo
}

// used in both get_xr_tree and composition tools
type CompositionInfo struct {
	Name     string
	Mode     string
	Pipeline []interface{}

	ResourceProviderList []ResourceInfo

	PipelineInfo []PipelineInfo
}

// used in annotate_reconcile
type ReconcileResult struct {
	Name      string
	Namespace string
	Kind      string
	Triggered bool
	Message   string
}

// check_provider_config
type ProviderConfig struct {
	Name                string
	Users               int64
	CredentialType      string
	SecretName          string
	CredentialNamespace string
	SecretExists        bool
	Ready               string
	Synced              string
	Conditions          []Condition
}

// condition
type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime string
	ObservedGeneration int64
}

// crossplane_info
type CrossplaneInfo struct {
	// In crossplane v2 claims are removed, and XR's made namespaced, to
	// remove another layer of abstraction.
	// core version
	Version      string // e.g. "v2.0.1"
	MajorVersion int    // 1 or 2

	// feature flags based on version
	HasMRDs          bool // v2 only
	HasOperations    bool // v2 only
	HasNamespacedXRs bool // v2 only
	HasNamespacedMRs bool // v2 only

	// XRD summary
	TotalXRDs      int
	NamespacedXRDs int
	ClusterXRDs    int

	// total provider
	NumberOfProvider int

	// provider summary
	Providers []ProviderInfo
}

type ProviderInfo struct {
	Name      string
	Version   string
	Health    bool
	Installed bool
	State     string
}

// debug_composition
type CompositionDebugResult struct {
	Name      string
	Mode      string
	ForKind   string
	Diagnosis Diagnosis
	Functions []FunctionStatus
	XRsUsing  []string // XRs currently using this composition
	Issues    []string
}

type FunctionStatus struct {
	Name      string
	Healthy   bool
	Installed bool
	Message   string
}

// debug_mr
type MRDebugResult struct {
	Name           string
	Namespace      string
	Kind           string
	Group          string
	Ready          string
	Synced         string
	Diagnosis      Diagnosis
	Conditions     []Condition
	Events         []EventInfo
	ProviderConfig *ProviderConfig
}

// debug_provider
type ProviderDebugResult struct {
	ProviderName    string
	Healthy         bool
	Installed       bool
	State           string
	Diagnosis       Diagnosis
	Conditions      []Condition
	Events          []EventInfo
	ProviderConfigs []ProviderConfigHealth
	AffectedMRs     int
}

// debug_xr
type Diagnosis struct {
	RootCause    string
	Severity     string
	AffectedPath string
	SuggestedFix string
	Details      []string
}

type DebugResult struct {
	XRName      string
	XRNamespace string
	XRReady     string
	XRSynced    string
	Diagnosis   Diagnosis
	Tree        *XRTreeInfo
	Events      []EventInfo
}

// dependency_graph
type DependencyNode struct {
	Name     string
	Kind     string
	Status   NodeStatus
	Message  string
	Children []*DependencyNode
}

// describe_xrd
type FieldInfo struct {
	Name        string
	Type        string
	Required    bool
	Default     interface{}
	Enum        []string
	Description string
	Children    []FieldInfo // for nested objects
}

type XRDDescription struct {
	Name           string
	Kind           string
	Group          string
	Version        string
	Scope          string
	Established    bool
	RequiredFields []FieldInfo
	OptionalFields []FieldInfo
	AllFields      []FieldInfo
	Summary        string
}

// events
type EventInfo struct {
	LastSeenTime time.Time
	LastSeen     string
	Type         string // Warning or Normal
	Reason       string
	Object       string
	Message      string
	Count        int32
}

// explain_composition

type TransformInfo struct {
	Type    string
	Map     map[string]interface{}
	Convert string
	Math    map[string]interface{}
}

type CombineVariable struct {
	FromFieldPath string
}

type CombineInfo struct {
	Strategy  string
	Format    string
	Variables []CombineVariable
}

type PatchInfo struct {
	Type          string
	FromFieldPath string
	ToFieldPath   string
	PatchSetName  string
	Policy        string
	Transforms    []TransformInfo
	Combine       *CombineInfo
}

type PatchSet struct {
	Name    string
	Patches []PatchInfo
}

type ResourceBlockInfo struct {
	Name           string
	APIVersion     string
	Kind           string
	ProviderConfig string
	DefaultValues  map[string]string
	Patches        []PatchInfo
}

type PipelineStep struct {
	StepName     string
	FunctionName string
	Resources    []ResourceBlockInfo
	PatchSets    []PatchSet
}

type CompositionExplanation struct {
	Name           string
	Mode           string
	ForKind        string
	ForAPIVersion  string
	Pipeline       []PipelineStep
	Summary        string
	TotalResources int
	TotalFunctions int
}

// get_managed_resource

type ManagedResourceInfo struct {
	UID       string
	Name      string
	Namespace string
	Kind      string
	Group     string
	Ready     string
	Synced    string
	Age       string
	Provider  string
}

type ManagedResourceDetail struct {
	ManagedResourceInfo
	Spec         map[string]interface{}
	Status       map[string]interface{}
	Conditions   []ConditionInfo
	Annotations  map[string]string
	CompositeRef string // which XR owns this MR
}

type ConditionInfo struct {
	Type    string
	Status  string
	Reason  string
	Message string
	Age     string
}

// get_xr_Tree

type MRTreeInfo struct {
	UID                string
	Name               string
	Kind               string
	Group              string
	Version            string
	Ready              string
	Synced             string
	ProviderConfigInfo ProviderConfigInfo
	ProviderConfigName string
}

type XRTreeInfo struct {
	UID             string
	XRName          string
	XRNamespace     string
	XRReady         string
	XRSynced        string
	XRKind          string
	XRFields        map[string]any
	CompositionInfo CompositionInfo
	MRs             []MRTreeInfo
}

// list_xrds
type XRDInfo struct {
	Name         string
	Kind         string
	Group        string
	Version      string
	Scope        string
	Established  bool
	XRCount      int      // how many XRs exist for this XRD
	Compositions []string // compositions that serve this XRD
}

type XRDList struct {
	Total      int
	Namespaced int
	Cluster    int
	XRDs       []XRDInfo
}

// list_Xrs

type XRObjectInfo struct {
	Group    string
	Version  string
	Resource string // plural name
	Kind     string
	Scope    string // Namespaced or Cluster
}

type XRInfo struct {
	Name           string
	Namespace      string
	Kind           string
	Ready          string
	Synced         string
	Message        string
	Age            string
	CompositionRef string
	Scope          string
	Group          string
}

type XRListResult struct {
	XRs      []XRInfo
	Warnings []string
}

// operations

// OperationResult holds a one-off operation
type OperationResult struct {
	Name           string
	Phase          string
	StartTime      string
	CompletionTime string
	Duration       string
	Message        string
}

// CronOperationResult holds a scheduled operation
type CronOperationResult struct {
	Name          string
	Schedule      string
	LastRunTime   string
	LastRunStatus string
	TotalRuns     int64
}

// WatchOperationResult holds an event-driven operation
type WatchOperationResult struct {
	Name          string
	WatchingKind  string
	WatchingName  string
	LastTriggered string
	TriggerCount  int64
	Status        string
}

// OperationsSummary is the full result returned to the agent
type OperationsSummary struct {
	Operations      []OperationResult
	CronOperations  []CronOperationResult
	WatchOperations []WatchOperationResult
}

// provider_health
type ProviderHealth struct {
	ProviderName    string
	Healthy         bool
	Installed       bool
	State           string
	Version         string
	Package         string
	HealthyMRs      int
	UnhealthyMRs    int
	ProviderConfigs []ProviderConfigHealth
	Conditions      []Condition
}

type ProviderConfigHealth struct {
	Name      string
	Namespace string
	Ready     string
	Synced    string
	SecretRef string
}

type ProviderHealthSummary struct {
	TotalProviders     int
	HealthyProviders   int
	UnhealthyProviders int
	Providers          []ProviderHealth
}

// validate_xr
type FieldConflict struct {
	XRField            string
	XRValue            string
	CompositionField   string
	CompositionDefault string
	PatchType          string
	ConflictType       string // "override", "invalid_region", "enum_violation", "type_mismatch"
	Warning            string
}

type ValidationResult struct {
	XRName        string
	XRNamespace   string
	Composition   string
	Valid         bool
	Conflicts     []FieldConflict
	MissingFields []string
	Warnings      []string
	Summary       string
}
