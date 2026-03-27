# helmx-demo

A demo Helm chart for testing helmx features. Deploys a simple nginx-based web application with configurable replicas, resource limits, autoscaling, and ingress.

## TL;DR

```bash
helm install my-demo ./charts/helmx-demo
```

## Introduction

This chart bootstraps a **helmx-demo** deployment on a Kubernetes cluster using the Helm package manager. It is designed to exercise all features of the helmx TUI:

- Values schema browser (`S`)
- README viewer (`r`)
- Version selector (`v`)
- Dry-run preview (`d`)
- Template to file (`t`)
- Install dialog with external editor (`e`)

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installing the Chart

Install the chart with release name `my-demo`:

```bash
# From local directory
helm install my-demo ./charts/helmx-demo

# With custom values
helm install my-demo ./charts/helmx-demo \
  --set replicaCount=3 \
  --set app.environment=production \
  --set app.message="Hello Production!"

# With values file
helm install my-demo ./charts/helmx-demo -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall my-demo
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Container image | `nginx` |
| `image.tag` | Image tag | `1.25` |
| `image.pullPolicy` | Pull policy | `IfNotPresent` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `80` |
| `ingress.enabled` | Enable ingress | `false` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Min replicas | `1` |
| `autoscaling.maxReplicas` | Max replicas | `10` |
| `app.environment` | Environment name | `development` |
| `app.logLevel` | Log level | `info` |
| `app.message` | Web page message | `Hello from helmx-demo!` |
| `app.features.darkMode` | Dark mode toggle | `false` |
| `app.features.metrics` | Metrics endpoint | `true` |
| `persistence.enabled` | Enable PVC | `false` |
| `persistence.size` | Storage size | `1Gi` |

## Examples

### Production deployment

```yaml
replicaCount: 3
app:
  environment: production
  logLevel: warn
  message: "Welcome to Production!"
  features:
    metrics: true
    healthCheck: true
resources:
  limits:
    cpu: 500m
    memory: 512Mi
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
```

### With ingress

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: demo.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: demo-tls
      hosts:
        - demo.example.com
```

### Minimal (dev)

```yaml
replicaCount: 1
app:
  environment: development
  logLevel: debug
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

## Health Checks

The chart exposes:
- **Liveness**: `GET /healthz`
- **Readiness**: `GET /readyz`

Configure via `healthCheck.liveness`, `healthCheck.readiness`, `healthCheck.initialDelaySeconds`.

## Autoscaling

Enable HPA with `autoscaling.enabled=true`. Scales based on CPU utilization (requires metrics-server).

## Security

- Runs as non-root (`runAsUser: 1000`)
- All capabilities dropped
- Privilege escalation disabled
