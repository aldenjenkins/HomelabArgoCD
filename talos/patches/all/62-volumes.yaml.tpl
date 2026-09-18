---
apiVersion: v1alpha1
kind: VolumeConfig
name: EPHEMERAL
{{- if hasKey .Node.Data "ephemeralMaxSize" }}
provisioning:
  maxSize: {{ .Node.Data.ephemeralMaxSize }}
{{- end }}
mount:
  # Note: need new secure default true option to be "secure: false" for longhorn REF https://github.com/siderolabs/talos/releases/tag/v1.14.0-beta.1
  secure: false
---
apiVersion: v1alpha1
kind: UserVolumeConfig
name: longhorn
provisioning:
  diskSelector:
    match: '{{ .Node.Data.longhornDiskSelector }}'
{{- if hasKey .Node.Data "longhornDiskMinSize" }}
  minSize: {{ .Node.Data.longhornDiskMinSize }}
{{- end }}
  maxSize: {{ .Node.Data.longhornDiskMaxSize }}
---
apiVersion: v1alpha1
kind: UserVolumeConfig
name: local-path-provisioner
provisioning:
  diskSelector:
    match: '{{ .Node.Data.localPathDiskSelector }}'
  minSize: {{ .Node.Data.localPathDiskMinSize }}
{{- if hasKey .Node.Data "localPathDiskMaxSize" }}
  maxSize: {{ .Node.Data.localPathDiskMaxSize }}
{{- end }}
{{- if hasKey .Node.Data "localPathGrow" }}
  grow: {{ .Node.Data.localPathGrow }}
{{- end }}
