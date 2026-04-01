package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ToolCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crossplane_mcp_server_tool_calls_total",
			Help: "Total number of tool calls",
		},
		[]string{"tool", "status"},
	)

	ToolCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "crossplane_mcp_server_tool_call_duration_seconds",
			Help:    "Duration of tool calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool"},
	)

	ToolCallErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crossplane_mcp_server_tool_errors_total",
			Help: "Total number of tool errors",
		},
		[]string{"tool", "error_type"},
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crossplane_mcp_server_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "crossplane_mcp_server_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)

	KubernetesOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crossplane_mcp_server_k8s_operations_total",
			Help: "Total number of Kubernetes operations",
		},
		[]string{"operation", "resource", "status"},
	)

	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "crossplane_mcp_server_active_requests",
			Help: "Number of currently active requests",
		},
	)
)

func RecordToolCall(tool string, status string, durationSeconds float64) {
	ToolCallsTotal.WithLabelValues(tool, status).Inc()
	ToolCallDuration.WithLabelValues(tool).Observe(durationSeconds)
}

func RecordToolError(tool string, errorType string) {
	ToolCallErrorsTotal.WithLabelValues(tool, errorType).Inc()
}

func RecordKubernetesOperation(operation, resource, status string) {
	KubernetesOperationsTotal.WithLabelValues(operation, resource, status).Inc()
}
