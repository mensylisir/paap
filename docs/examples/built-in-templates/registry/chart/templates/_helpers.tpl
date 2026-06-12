{{- define "registry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "registry.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "registry.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "registry.serviceAccountName" -}}
{{- if .Values.serviceAccount.name -}}
{{- .Values.serviceAccount.name -}}
{{- else -}}
{{- include "registry.fullname" . -}}
{{- end -}}
{{- end -}}

{{- define "registry.tlsSecretName" -}}
{{- if .Values.tls.secretName -}}
{{- .Values.tls.secretName -}}
{{- else -}}
{{- printf "%s-tls" (include "registry.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
