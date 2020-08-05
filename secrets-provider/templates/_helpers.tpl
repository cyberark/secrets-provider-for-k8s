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
