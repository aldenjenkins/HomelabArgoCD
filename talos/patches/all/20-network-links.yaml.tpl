{{- /*
Node networking as Talos 1.13+ typed network documents. The MAC-matched
link is enslaved to a single-link active-backup bond so every consumer
(Cilium devices, VLANs, the VIP, metrics) sees a stable interface name,
bond0, regardless of kernel NIC naming; a second NIC can join the bond
later without renaming anything. The bond name is also referenced by
`devices` in the cilium HelmRelease.

bond0 stays on DHCP; only the tagged VLANs carry static addresses, and their
last octet always matches bond0's, so it is derived here.
*/ -}}
{{- $octet := last (splitList "." (printf "%s" .Node.IP)) }}
---
apiVersion: v1alpha1
kind: LinkAliasConfig
name: ethSel0
selector:
  match: glob("{{ .Node.Data.macAddr }}", mac(link.hardware_addr))
{{- if eq .Node.Role "control-plane" }}
---
apiVersion: v1alpha1
kind: Layer2VIPConfig
name: 192.168.123.50
link: ethSel0
{{- end }}
---
apiVersion: v1alpha1
kind: LinkConfig
name: ethSel0
addresses:
  - address: {{ .Node.IP }}/24
routes:
  - gateway: 192.168.123.1
