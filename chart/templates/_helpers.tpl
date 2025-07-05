{{/*
Expand the name of the chart.
*/}}
{{- define "microservice.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "microservice.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "microservice.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "microservice.labels" -}}
helm.sh/chart: {{ include "microservice.chart" . }}
{{ include "microservice.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "microservice.selectorLabels" -}}
app.kubernetes.io/name: {{ include "microservice.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


{{/*
Multi-service helpers
*/}}

{{/*
Create a service-specific name
*/}}
{{- define "microservice.serviceName" -}}
{{- $serviceName := .service.name -}}
{{- $globalName := include "microservice.fullname" .root -}}
{{- printf "%s-%s" $globalName $serviceName | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Create service-specific labels
*/}}
{{- define "microservice.serviceLabels" -}}
{{ include "microservice.labels" .root }}
app.kubernetes.io/component: {{ .service.name }}
{{- end }}

{{/*
Create service-specific selector labels
*/}}
{{- define "microservice.serviceSelectorLabels" -}}
{{ include "microservice.selectorLabels" .root }}
app.kubernetes.io/component: {{ .service.name }}
{{- end }}


{{/*
Get effective services list
*/}}
{{- define "microservice.effectiveServices" -}}
{{- .Values.services | toYaml -}}
{{- end }}

{{/*
Get service config by merging defaults with service overrides
*/}}
{{- define "microservice.getServiceConfig" -}}
{{- $service := .service -}}
{{- $defaults := .root.Values.defaults -}}
{{- $result := mergeOverwrite $defaults $service -}}
{{- $result -}}
{{- end }}

{{/*
Create the name of the service account to use for a specific service
*/}}
{{- define "microservice.serviceAccountNameForService" -}}
{{- $config := include "microservice.getServiceConfig" . | fromYaml -}}
{{- if $config.serviceAccount.create }}
{{- if $config.serviceAccount.name -}}
{{- $config.serviceAccount.name }}
{{- else -}}
{{- include "microservice.serviceName" . }}
{{- end -}}
{{- else }}
{{- default "default" $config.serviceAccount.name }}
{{- end }}
{{- end }}