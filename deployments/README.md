# Kubernetes Deployment Manifests

This directory contains Kubernetes manifests for deploying the all-in-one application.

## Overview

- **deployment.yaml**: Main application deployment with 2 replicas
- **service.yaml**: ClusterIP service for internal access
- **configmap.yaml**: Application configuration
- **pvc.yaml**: Persistent volume claim for SQLite database
- **ingress.yaml**: Ingress for external access
- **kustomization.yaml**: Kustomize configuration for managing manifests

## Prerequisites

1. Kubernetes cluster (v1.25+)
2. kubectl configured
3. Ingress controller (nginx recommended)
4. Storage class for PVC

## Quick Deploy

### Using kubectl

```bash
# Deploy all resources
kubectl apply -f deployments/

# Check deployment status
kubectl get pods -l app=all-in-one
kubectl get svc all-in-one
kubectl get ingress all-in-one
```

### Using Kustomize

```bash
# Deploy using kustomize
kubectl apply -k deployments/

# Preview changes
kubectl kustomize deployments/
```

## Configuration Steps

### 1. Update Image Registry

Edit `deployments/deployment.yaml`:
```yaml
image: YOUR_REGISTRY/all-in-one:latest
```

Edit `deployments/kustomization.yaml`:
```yaml
images:
- name: YOUR_REGISTRY/all-in-one
  newTag: latest
```

### 2. Update Ingress Domain

Edit `deployments/ingress.yaml`:
```yaml
spec:
  rules:
  - host: all-in-one.yourdomain.com
```

### 3. Configure Storage

Edit `deployments/pvc.yaml` to match your cluster's storage class:
```yaml
spec:
  storageClassName: standard  # or your storage class name
```

### 4. Set Secrets (Optional)

Create a secret for sensitive data:
```bash
kubectl create secret generic all-in-one-secrets \
  --from-literal=jwt-secret=YOUR_JWT_SECRET
```

Update deployment to use the secret:
```yaml
env:
- name: JWT_SECRET
  valueFrom:
    secretKeyRef:
      name: all-in-one-secrets
      key: jwt-secret
```

## Health Checks

The deployment includes:
- **Liveness probe**: Checks if the app is running (restarts if fails)
- **Readiness probe**: Checks if the app is ready to serve traffic

Endpoint: `GET /api/v1/health`

You'll need to implement this endpoint in your Go backend if it doesn't exist yet.

## Resource Management

Default resource limits:
- **Requests**: 128Mi memory, 100m CPU
- **Limits**: 512Mi memory, 500m CPU

Adjust based on your workload in `deployment.yaml`.

## Scaling

```bash
# Manual scaling
kubectl scale deployment all-in-one --replicas=3

# Auto-scaling (requires metrics-server)
kubectl autoscale deployment all-in-one --min=2 --max=10 --cpu-percent=80
```

## Monitoring

```bash
# View logs
kubectl logs -f deployment/all-in-one

# View logs from specific pod
kubectl logs -f <pod-name>

# Execute commands in pod
kubectl exec -it <pod-name> -- /bin/sh
```

## Troubleshooting

### Pod not starting

```bash
# Check pod status
kubectl describe pod <pod-name>

# Check events
kubectl get events --sort-by='.lastTimestamp'
```

### Database issues

```bash
# Check PVC
kubectl describe pvc all-in-one-data

# Verify volume mount
kubectl exec -it <pod-name> -- ls -la /data
```

### Image pull errors

Ensure you have proper image pull secrets configured:
```bash
kubectl create secret docker-registry regcred \
  --docker-server=YOUR_REGISTRY \
  --docker-username=YOUR_USERNAME \
  --docker-password=YOUR_PASSWORD

# Update deployment to use the secret
spec:
  imagePullSecrets:
  - name: regcred
```

## Clean Up

```bash
# Delete all resources
kubectl delete -f deployments/

# Or using kustomize
kubectl delete -k deployments/
```

## Production Considerations

1. **Use namespaces**: Deploy to a dedicated namespace
2. **Enable TLS**: Uncomment TLS section in ingress.yaml
3. **Set resource limits**: Adjust based on load testing
4. **Use secrets**: Never commit sensitive data
5. **Database backups**: Implement backup strategy for SQLite
6. **Monitoring**: Set up Prometheus/Grafana
7. **Logging**: Use log aggregation (ELK, Loki)
8. **Multiple replicas**: Ensure stateless design for scaling
