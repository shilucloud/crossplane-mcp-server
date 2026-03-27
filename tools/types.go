package tools

// used in get_xr_tree and composition tool
type ProviderConfigInfo struct {
	Name   string
	Group  string
	Ready  string
	Synced string
	Kind   string
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

// used in both get_xr_tree and composition tools
type CompositionInfo struct {
	Name     string
	Mode     string
	Pipeline []interface{}

	ResourceProviderList []ResourceInfo
}
