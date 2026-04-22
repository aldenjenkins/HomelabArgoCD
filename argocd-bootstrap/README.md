# Bootstrap the cluster

with your age key setup for sops, run this from the argocd-bootstrap directory:

```
kustomize build --enable-alpha-plugins --enable-exec . | k apply -f -
```

Then add the age key to the argocd namespace wherever your age key is located on your machine:

```
cat age.key | kubectl create secret generic sops-age --namespace=argocd --from-file=keys.txt=/dev/stdin
```

For gethomepage.dev app's readonly secret on a fresh build, go into the argocd UI and create a token for that user who is named "readonly"
