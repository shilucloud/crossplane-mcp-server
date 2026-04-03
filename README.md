# crossplane-mcp-server

An MCP server that exposes Crossplane resources to any MCP-compatible AI agent. Lets agents inspect, debug, and manage XRs, compositions, providers, and managed resources and more through natural language.

---

## Prerequisites

| Requirement | Details |
|---|---|
| `Kubernetes` cluster | with Crossplane installed |
| `kubectl` | configured with cluster access |
| `helm` | >= 3 |
| `MCP-compatible agent` | kagent(tested), Claude, etc. |

---

## Installation

### Via Helm (from this repo)
```bash
helm install crossplane-mcp-server ./charts/crossplane-mcp-server \
  --namespace crossplane-system \
  --create-namespace
```

### Via manifest
```bash
kubectl apply -f manifest.yaml
```

### Via binary 
```bash 
go build -o bin/server cmd/server/main.go
```

---

## Configuration

Edit `charts/crossplane-mcp-server/values.yaml`:
```yaml
image:
  repository: ghcr.io/<your-org>/crossplane-mcp-server
  tag: latest

replicaCount: 1
```

---

## Available Tools

| Tool | Description |
|---|---|
| `get_xr_tree` | Full resource tree of a composite resource |
| `list_xrs` | List all composite resources |
| `list_xrds` | List all composite resource definitions |
| `describe_xrd` | Describe a specific XRD |
| `validate_xr` | Validate a composite resource against its XRD |
| `debug_xr` | Debug a composite resource |
| `debug_mr` | Debug a managed resource |
| `debug_composition` | Debug a composition |
| `debug_provider` | Debug a Crossplane provider |
| `explain_composition` | Explain what a composition does in plain language |
| `get_managed_resource` | Get details of a managed resource |
| `dependency_graph` | Generate a dependency graph for resources |
| `provider_health` | Check provider health status |
| `check_provider_config` | Validate a provider config |
| `events` | Fetch Kubernetes events for a resource |
| `condition` | Get conditions of a resource |
| `annotate_reconcile` | Trigger reconciliation via annotation |

---

## Development

Requires [Taskfile](https://taskfile.dev).
```bash
task --list       # see all available tasks
task build        # build the binary
task test         # run tests
task run          # run locally
```

Build the Docker image:
```bash
docker build -t crossplane-mcp-server .
```

---

## Repository Structure
```
.
├── charts/crossplane-mcp-server/   # Helm chart
├── cmd/server/                     # Server entrypoint
├── internal/
│   ├── logging/                    # Structured logging
│   ├── metrics/                    # Metrics instrumentation
│   └── tools/                      # Tool registration and Kubernetes clients
├── tools/                          # MCP tool implementations
├── Dockerfile
├── manifest.yaml                   # Raw Kubernetes manifest
└── Taskfile.yml
```

---

## Helm Chart Releases

Packaged charts are in `docs/` and served as a Helm repo. To use:
```bash
helm repo add crossplane-mcp-server https://shilucloud.github.io/crossplane-mcp-server
helm repo update
```