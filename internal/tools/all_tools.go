package tools

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func AddToolsToServer(server *mcp.Server) {
	for _, addToolFunc := range toolsToAdd {
		addToolFunc(server)
	}
}

var toolsToAdd []func(server *mcp.Server)

// registerTool registers a typed tool using the v1.4.1 mcp.AddTool API.
// The handler signature expected by mcp.AddTool is:
//
//	func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, O, error)
func registerTool[I any](name, description string, fn func(ctx context.Context, input I) (any, error)) {
	toolsToAdd = append(toolsToAdd, func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        name,
			Description: description,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, any, error) {
			result, err := fn(ctx, input)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "error: " + err.Error()}},
					IsError: true,
				}, nil, nil
			}
			out, jsonErr := json.MarshalIndent(result, "", "  ")
			if jsonErr != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "error marshaling result: " + jsonErr.Error()}},
					IsError: true,
				}, nil, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
			}, nil, nil
		})
	})
}
