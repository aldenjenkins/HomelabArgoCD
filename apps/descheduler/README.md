# Use this

```{bash}
helm template descheduler descheduler/descheduler --namespace kube-system --values values.yaml >! descheduler.yaml
```
