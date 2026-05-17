{{- define "pingsvc.name" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "pingsvc.labels" -}}
app.kubernetes.io/name: {{ include "pingsvc.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "pingsvc.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pingsvc.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
