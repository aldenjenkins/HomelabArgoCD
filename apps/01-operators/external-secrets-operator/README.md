ESO only provides helm install options and only supports kustomize for CRDS.

`manifests.yaml` is generated locally with:

```
helm repo update external-secrets
helm template external-secrets external-secrets/external-secrets --namespace external-secrets-operator-system --set installCRDs=false --set serviceMonitor.enabled=true --api-versions monitoring.coreos.com/v1 | yq eval 'del(.metadata.labels."helm.sh/chart") | del(.metadata.labels."app.kubernetes.io/managed-by")' - >! manifests.yaml
```
