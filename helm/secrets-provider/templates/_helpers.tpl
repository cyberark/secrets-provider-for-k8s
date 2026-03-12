{{/*
Return the most recent RBAC API available
*/}}
{{- define "conjur.rbac-api" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" }}
{{- printf "rbac.authorization.k8s.io/v1" -}}
{{- else if .Capabilities.APIVersions.Has "v1" }}
{{- printf "v1" -}}
{{- end }}
{{- end }}

{{/*
Returns "true" when cluster-scoped RBAC is needed (ClusterRole/ClusterRoleBinding).
Only in standalone mode when namespaceAllowlist is "*" or multiple comma-separated namespaces.
Application mode uses a single namespace, so Role always suffices.
*/}}
{{- define "conjur.useClusterScopedRBAC" -}}
{{- $standalone := eq (default "application" .Values.containerMode) "standalone" -}}
{{- $allowlist := trim (default "" .Values.standalone.namespaceAllowlist) -}}
{{- $multiNs := or (eq $allowlist "*") (contains "," $allowlist) -}}
{{- if and $standalone $multiNs }}true{{ else }}false{{ end -}}
{{- end }}
