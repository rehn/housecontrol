{{- define "housecontrol.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "housecontrol.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "housecontrol.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "housecontrol.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "housecontrol.selectorLabels" -}}
app.kubernetes.io/name: {{ include "housecontrol.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "housecontrol.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "housecontrol.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "housecontrol.image" -}}
{{- $repo := required "set image.repository, e.g. docker.io/<your-dockerhub-user>/housecontrol" .Values.image.repository }}
{{- printf "%s:%s" $repo (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}

{{/* Passwords that go into the chart's own Secret (not when an existing Secret is used). */}}
{{- define "housecontrol.secretData" -}}
{{- if and .Values.auth.password (not .Values.auth.existingSecret) }}
ADMIN_PASSWORD: {{ .Values.auth.password | quote }}
{{- end }}
{{- if and .Values.mqtt.password (not .Values.mqtt.existingSecret) }}
MQTT_PASSWORD: {{ .Values.mqtt.password | quote }}
{{- end }}
{{- end }}

{{- define "housecontrol.trustProxy" -}}
{{- if eq (toString .Values.trustProxy) "" }}
{{- .Values.httpRoute.enabled | toString }}
{{- else }}
{{- .Values.trustProxy | toString }}
{{- end }}
{{- end }}

{{- define "housecontrol.claimName" -}}
{{- .Values.persistence.existingClaim | default (include "housecontrol.fullname" .) }}
{{- end }}
