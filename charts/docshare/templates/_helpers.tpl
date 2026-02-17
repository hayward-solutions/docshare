{{- define "docshare.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "docshare.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "docshare.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "docshare.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "docshare.selectorLabels" -}}
app.kubernetes.io/name: {{ include "docshare.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "docshare.labels" -}}
helm.sh/chart: {{ include "docshare.chart" . }}
{{ include "docshare.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "docshare.apiUrl" -}}
{{- printf "http://%s-api:%v" (include "docshare.fullname" .) .Values.api.service.port -}}
{{- end -}}

{{- define "docshare.postgresHost" -}}
{{- if .Values.postgresql.enabled -}}
{{- printf "%s-postgresql" .Release.Name -}}
{{- else -}}
{{- .Values.externalDatabase.host -}}
{{- end -}}
{{- end -}}

{{- define "docshare.postgresPort" -}}
{{- if .Values.postgresql.enabled -}}
5432
{{- else -}}
{{- .Values.externalDatabase.port -}}
{{- end -}}
{{- end -}}

{{- define "docshare.s3Endpoint" -}}
{{- if .Values.s3.endpoint -}}
{{- .Values.s3.endpoint -}}
{{- else -}}
{{- printf "s3.%s.amazonaws.com" .Values.s3.region -}}
{{- end -}}
{{- end -}}

{{- define "docshare.gotenbergUrl" -}}
{{- if .Values.gotenberg.enabled -}}
{{- printf "http://%s-gotenberg:%v" (include "docshare.fullname" .) .Values.gotenberg.service.port -}}
{{- end -}}
{{- end -}}

{{- define "docshare.externalFrontendUrl" -}}
{{- if .Values.api.env.frontendUrl -}}
{{- .Values.api.env.frontendUrl -}}
{{- else if .Values.ingress.enabled -}}
{{- $host := (index .Values.ingress.hosts 0).host -}}
{{- if .Values.ingress.tls -}}https://{{ $host }}{{- else -}}http://{{ $host }}{{- end -}}
{{- else -}}
http://localhost:3001
{{- end -}}
{{- end -}}

{{- define "docshare.externalApiUrl" -}}
{{- if .Values.api.env.backendUrl -}}
{{- .Values.api.env.backendUrl -}}
{{- else if .Values.ingress.enabled -}}
{{- $host := (index .Values.ingress.hosts 0).host -}}
{{- if .Values.ingress.tls -}}https://{{ $host }}/api{{- else -}}http://{{ $host }}/api{{- end -}}
{{- else -}}
http://localhost:8080/api
{{- end -}}
{{- end -}}

{{- define "docshare.secretName" -}}
{{- if .Values.api.existingSecret -}}
{{- .Values.api.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "docshare.fullname" .) -}}
{{- end -}}
{{- end -}}
