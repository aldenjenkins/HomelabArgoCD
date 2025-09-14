# Bootstrap the cluster

with your age key setup for sops, run this from the argocd-bootstrap directory:

```
kustomize build --enable-alpha-plugins --enable-exec . | k apply -f -
```

Then add the age key to the argocd namespace wherever your age key is located on your machine:

```
cat age.key | kubectl create secret generic sops-age --namespace=argocd --from-file=keys.txt=/dev/stdin
```

