# crossplane-agent Helm Chart

MCP server for Crossplane management in Kubernetes.

## Quick Start

```bash
# Add Helm repo (once the chart is published)
helm repo add crossplane-agent https://ghcr.io/<owner>/crossplane-agent
helm repo update

# Install from repo
helm install crossplane-agent crossplane-agent/crossplane-agent

# Or install from local chart
helm install crossplane-agent ./charts/crossplane-agent
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Container image repository | `shilcloud/crossplane-agent` |
| `image.tag` | Container image tag | `0.0.4` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `probes.enabled` | Enable liveness/readiness probes | `true` |
| `rbac.create` | Create ServiceAccount and ClusterRole | `true` |

## Examples

```bash
# Use GHCR image
helm install crossplane-agent ./charts/crossplane-agent \
  --set image.repository=ghcr.io/<owner>/crossplane-agent

# With custom tag
helm install crossplane-agent ./charts/crossplane-agent \
  --set image.tag=v1.2.3

# High availability
helm install crossplane-agent ./charts/crossplane-agent \
  --set replicaCount=3
```

## Image Registries

- **Docker Hub:** `shilcloud/crossplane-agent:<tag>`
- **GHCR:** `ghcr.io/<owner>/crossplane-agent:<tag>`
