{{- define "eureka.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "eureka.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "eureka.name" . -}}
{{- end -}}
{{- end -}}

{{- define "eureka.labels" -}}
app.kubernetes.io/name: {{ include "eureka.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: registry
paap.io/service-type: eureka
{{- end -}}

{{- define "eureka.serviceAccountName" -}}
{{- if .Values.serviceAccount.name -}}
{{- .Values.serviceAccount.name -}}
{{- else -}}
{{- include "eureka.fullname" . -}}
{{- end -}}
{{- end -}}
