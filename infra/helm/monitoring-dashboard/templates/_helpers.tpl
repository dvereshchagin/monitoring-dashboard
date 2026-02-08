{{- define "monitoring-dashboard.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "monitoring-dashboard.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "monitoring-dashboard.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "monitoring-dashboard.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "monitoring-dashboard.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "monitoring-dashboard.gatewayName" -}}
{{- printf "%s-gateway" (include "monitoring-dashboard.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "monitoring-dashboard.gatewayFullname" -}}
{{- printf "%s-gateway" (include "monitoring-dashboard.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "monitoring-dashboard.gatewayServiceAccountName" -}}
{{- if .Values.gateway.serviceAccount.create -}}
{{- default (include "monitoring-dashboard.gatewayFullname" .) .Values.gateway.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.gateway.serviceAccount.name -}}
{{- end -}}
{{- end -}}
