{{/*
Expand the name of the chart.
*/}}
{{- define "pharos.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pharos.fullname" -}}
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

{{- define "pharos.postgres.fullname" -}}
{{ template "pharos.fullname" . }}-postgres
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "pharos.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pharos.labels" -}}
helm.sh/chart: {{ include "pharos.chart" . }}
{{ include "pharos.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "pharos.postgres.labels" -}}
{{ include "pharos.labels" . }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pharos.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pharos.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "pharos.postgres.selectorLabels" -}}
{{ include "pharos.selectorLabels" . }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pharos.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pharos.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "pharos.postgres.serviceAccountName" -}}
{{- if .Values.postgres.serviceAccount.create }}
{{- default (include "pharos.postgres.fullname" .) .Values.postgres.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.postgres.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "pharos.dbEnvVars" -}}
{{- if .Values.postgres.enabled }}
{{- if .Values.postgres.auth.existingSecret }}
- name: PHAROS_DATABASE_USERNAME
  valueFrom:
    secretKeyRef:
    name: {{ .Values.postgres.auth.existingSecret }}
    key: username
- name: PHAROS_DATABASE_PASSWORD
  valueFrom:
    secretKeyRef:
    name: {{ .Values.postgres.auth.existingSecret }}
    key: password
{{- else }}
- name: PHAROS_DATABASE_USERNAME
  value: "{{ .Values.postgres.auth.username }}"
- name: PHAROS_DATABASE_PASSWORD
  value: "{{ .Values.postgres.auth.password }}"
{{- end }}
- name: PHAROS_DATABASE
  value: "{{ .Values.postgres.auth.database }}"
{{- else }}
- name: PHAROS_DATABASE_DSN
  valueFrom:
    secretKeyRef:
    name: {{ .Values.externalDatabase.secret }}
    key: dsn
{{- end }}
{{- end }}