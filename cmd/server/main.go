package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shilucloud/crossplane-mcp-server/internal/logging"
	"github.com/shilucloud/crossplane-mcp-server/internal/tools"
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
		mux := http.NewServeMux()
		mux.Handle("/health", healthHandler())
		mux.Handle("/ready", readyHandler())
		mux.Handle("/metrics", promhttp.Handler())

		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)
		mux.Handle("/mcp", mcpHandler)

		srv := &http.Server{
			Addr:    *httpAddr,
			Handler: mux,
		}

		go func() {
			logging.Info("MCP server listening", "address", *httpAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logging.Error("HTTP server error", "error", err.Error())
			}
		}()

		return gracefulShutdown(srv)
	}

	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
}

func readyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if tools.IsReady() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready"}`))
		}
	})
}

func gracefulShutdown(srv *http.Server) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logging.Info("server exited gracefully")
	return nil
}
