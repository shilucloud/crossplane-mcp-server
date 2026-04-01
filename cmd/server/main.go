package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shilucloud/crossplane-agent/internal/logging"
	"github.com/shilucloud/crossplane-agent/internal/tools"
)

var (
	httpAddr = flag.String("http", "", "if set, use streamable HTTP to serve MCP (on this address), instead of stdin/stdout")
	logLevel = flag.String("log-level", "info", "log level: debug, info, warn, error")
)

func main() {
	flag.Parse()
	logging.Init(*logLevel)
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// init k8s clients
	if err := tools.InitClients(); err != nil {
		return fmt.Errorf("error initializing k8s clients: %w", err)
	}

	// create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "crossplane-agent",
		Version: "0.1.0",
	}, nil)

	// register tools
	tools.AddToolsToServer(server)

	// start server
	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)
		logging.Info("MCP server listening", "address", *httpAddr)
		return http.ListenAndServe(*httpAddr, handler)
	}

	return server.Run(context.Background(), &mcp.StdioTransport{})

}
