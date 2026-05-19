{{- define "insider-service.name" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "insider-service.labels" -}}
app.kubernetes.io/name: {{ include "insider-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "insider-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "insider-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
