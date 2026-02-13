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

{{- define "docshare.backendUrl" -}}
{{- printf "http://%s-backend:%v" (include "docshare.fullname" .) .Values.backend.service.port -}}
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

{{- define "docshare.minioEndpoint" -}}
{{- if .Values.minio.enabled -}}
{{- printf "%s-minio:9000" .Release.Name -}}
{{- else -}}
{{- .Values.externalMinio.endpoint -}}
{{- end -}}
{{- end -}}

{{- define "docshare.gotenbergUrl" -}}
{{- if .Values.gotenberg.enabled -}}
{{- printf "http://%s-gotenberg:%v" (include "docshare.fullname" .) .Values.gotenberg.service.port -}}
{{- end -}}
{{- end -}}

{{- define "docshare.secretName" -}}
{{- if .Values.backend.existingSecret -}}
{{- .Values.backend.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "docshare.fullname" .) -}}
{{- end -}}
{{- end -}}
