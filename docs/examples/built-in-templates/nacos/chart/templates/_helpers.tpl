{{- define "nacos.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nacos.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "nacos.name" . -}}
{{- end -}}
{{- end -}}

{{- define "nacos.labels" -}}
app.kubernetes.io/name: {{ include "nacos.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: registry-config
paap.io/service-type: nacos
{{- end -}}

{{- define "nacos.serviceAccountName" -}}
{{- if .Values.serviceAccount.name -}}
{{- .Values.serviceAccount.name -}}
{{- else -}}
{{- include "nacos.fullname" . -}}
{{- end -}}
{{- end -}}
