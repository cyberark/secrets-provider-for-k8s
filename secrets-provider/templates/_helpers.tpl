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
Return the most recent Workload API available
*/}}
{{- define "conjur.deploy-api" -}}
{{- if .Capabilities.APIVersions.Has "batch/v1" }}
{{- printf "batch/v1" -}}
{{- else if .Capabilities.APIVersions.Has "apps/v1" }}
{{- printf "apps/v1" -}}
{{- end }}
{{- end }}
