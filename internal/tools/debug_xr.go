package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	crossplane "github.com/shilucloud/crossplane-agent/tools"
)

func init() {
	registerTool(DebugXR())
}

type DebugXRParams struct {
	Group     string `json:"group"     description:"XR API group e.g. platform.example.com"`
	Version   string `json:"version"   description:"XR version e.g. v1alpha1"`
	Resource  string `json:"resource"  description:"XR plural resource name e.g. xbuckets"`
	Name      string `json:"name"      description:"Name of the XR"`
	Namespace string `json:"namespace" description:"Namespace of the XR (empty for cluster-scoped)"`
}

func DebugXR() MCPTool[DebugXRParams, any] {
	return MCPTool[DebugXRParams, any]{
		Name:        "debug_xr",
		Description: "Debug a Crossplane XR. Diagnoses root cause of failures by walking the full resource tree — XR → Composition → MRs → ProviderConfig. Returns structured diagnosis with severity, root cause, affected path and suggested fix.",
		Handler: func(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[DebugXRParams]) (*mcp.CallToolResultFor[any], error) {
			p := params.Arguments
			result, err := crossplane.DebugXR(ctx, DynamicClient, Clientset,
				p.Group, p.Version, p.Resource, p.Name, p.Namespace)
			if err != nil {
				return mcpError(err.Error()), nil
			}
			return mcpResult(result)
		},
	}
}
