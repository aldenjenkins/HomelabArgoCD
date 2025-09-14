argocd will use the templatedistiofiles.yaml which whenever you update meshConfig.yaml you should run  

```
istioctl manifest generate -f meshConfig.yaml >! templatedistiofiles.yaml
```

then commit templatedistiofiles.yaml to the repo
