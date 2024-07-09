{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "vault-unsealer.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "vault-unsealer.fullname" -}}
{{- $name := .Chart.Name -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "vault-unsealer.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "vault-unsealer.labels" -}}
helm.sh/chart: {{ include "vault-unsealer.chart" . }}
helm.sh/namespace: {{ .Release.Namespace }}
{{ include "vault-unsealer.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "vault-unsealer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vault-unsealer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "vault-unsealer.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "vault-unsealer.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Create the name of the role to use
*/}}
{{- define "vault-unsealer.roleName" -}}
{{- if .Values.rbac.create -}}
    {{ default (include "vault-unsealer.fullname" .) .Values.rbac.roleName }}
{{- else -}}
    {{ default "default" .Values.rbac.roleName }}
{{- end -}}
{{- end -}}

{{- define "vault-unsealer.mounts" -}}
  {{- range .Values.volumes }}
            - name: {{ .name }}
              readOnly: true
              mountPath: {{ .path }}/{{ .name }}
  {{- end }}
{{- end -}}

{{- define "vault-unsealer.volumes" -}}
  {{- range .Values.volumes }}
        - name: {{ .name }}
          {{ .type }}:
          {{- if (eq .type "configMap") }}
            name: {{ .name }}
          {{- else if (eq .type "secret") }}
            secretName: {{ .name }}
          {{- end }}
            defaultMode: {{ .defaultMode | default 420 }}
  {{- end }}
{{- end -}}