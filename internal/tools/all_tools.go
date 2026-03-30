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

func registerTool[I, O any](tool MCPTool[I, O]) {
	toolsToAdd = append(toolsToAdd, func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{Name: tool.Name, Description: tool.Description}, tool.Handler)
	})
}

type MCPTool[I, O any] struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[I]) (*mcp.CallToolResultFor[O], error)
}

// add these at the bottom of all_tools.go

func mcpResult(v interface{}) (*mcp.CallToolResultFor[any], error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpError(err.Error()), nil
	}
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
	}, nil
}

func mcpError(msg string) *mcp.CallToolResultFor[any] {
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: "error: " + msg}},
		IsError: true,
	}
}
