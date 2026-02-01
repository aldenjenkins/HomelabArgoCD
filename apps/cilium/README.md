Required talos machineconfig patch for cilium kube-proxy replacement:

cluster:
  network:
    cni:
      name: none
  proxy:
    disabled: true
