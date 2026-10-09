{{/*
Expand the name of the chart.
*/}}
{{- define "ai-kbs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a fully qualified app name. Truncated at 63 chars because some K8s
name fields are limited.
*/}}
{{- define "ai-kbs.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "ai-kbs.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "ai-kbs.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ai-kbs
{{- end -}}

{{/*
Selector labels (used in matchLabels).
*/}}
{{- define "ai-kbs.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai-kbs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Component selector (used for per-component matchLabels).
Usage: include "ai-kbs.componentSelector" (dict "ctx" . "component" "backend")
*/}}
{{- define "ai-kbs.componentSelector" -}}
{{- $ctx := .ctx -}}
{{- $component := .component -}}
{{- $fullname := include "ai-kbs.fullname" $ctx -}}
app.kubernetes.io/name: {{ $fullname }}-{{ $component }}
app.kubernetes.io/instance: {{ $ctx.Release.Name }}
{{- end -}}

{{/*
Component labels (used on Pod/Deployment/Service).
Usage: include "ai-kbs.componentLabels" (dict "ctx" . "component" "backend")
*/}}
{{- define "ai-kbs.componentLabels" -}}
{{- $ctx := .ctx -}}
{{- $component := .component -}}
{{- $fullname := include "ai-kbs.fullname" $ctx -}}
helm.sh/chart: {{ printf "%s-%s" $ctx.Chart.Name $ctx.Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "ai-kbs.componentSelector" (dict "ctx" $ctx "component" $component) }}
app.kubernetes.io/version: {{ $ctx.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ $ctx.Release.Service }}
app.kubernetes.io/part-of: ai-kbs
app.kubernetes.io/component: {{ $component }}
{{- end -}}

{{/*
Image reference.
Usage: include "ai-kbs.image" (dict "image" .Values.backend.image "global" .Values.global)
*/}}
{{- define "ai-kbs.image" -}}
{{- $image := .image -}}
{{- $global := .global -}}
{{- $pullPolicy := $image.pullPolicy | default $global.imagePullPolicy -}}
{{- printf "%s:%s" $image.repository $image.tag -}}
{{- end -}}
