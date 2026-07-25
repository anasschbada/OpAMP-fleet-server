{{/*
Base name for resources, honoring nameOverride/fullnameOverride.
*/}}
{{- define "opamp-fleet-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "opamp-fleet-server.fullname" -}}
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

{{- define "opamp-fleet-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every resource.
*/}}
{{- define "opamp-fleet-server.labels" -}}
helm.sh/chart: {{ include "opamp-fleet-server.chart" . }}
{{ include "opamp-fleet-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels for the SERVER workload specifically.
*/}}
{{- define "opamp-fleet-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "opamp-fleet-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: server
{{- end -}}

{{- define "opamp-fleet-server.serviceAccountName" -}}
{{- if .Values.server.serviceAccount.create -}}
{{- default (include "opamp-fleet-server.fullname" .) .Values.server.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.server.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
UI naming/labels: a distinct name+component so the server and UI
Deployments never collide on selectors within the same release.
*/}}
{{- define "opamp-fleet-server.ui.fullname" -}}
{{- printf "%s-ui" (include "opamp-fleet-server.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "opamp-fleet-server.ui.labels" -}}
helm.sh/chart: {{ include "opamp-fleet-server.chart" . }}
{{ include "opamp-fleet-server.ui.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "opamp-fleet-server.ui.selectorLabels" -}}
app.kubernetes.io/name: {{ include "opamp-fleet-server.name" . }}-ui
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: ui
{{- end -}}

{{- define "opamp-fleet-server.ui.serviceAccountName" -}}
{{- if .Values.ui.serviceAccount.create -}}
{{- default (include "opamp-fleet-server.ui.fullname" .) .Values.ui.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.ui.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Secret + key references for the two token pools, resolving to either the
user-supplied existingSecret or the chart-managed one this chart creates
in secret-agent-tokens.yaml/secret-api-tokens.yaml.
*/}}
{{- define "opamp-fleet-server.agentTokensSecretName" -}}
{{- .Values.auth.agentTokens.existingSecret | default (printf "%s-agent-tokens" (include "opamp-fleet-server.fullname" .)) -}}
{{- end -}}

{{- define "opamp-fleet-server.apiTokensSecretName" -}}
{{- .Values.auth.apiTokens.existingSecret | default (printf "%s-api-tokens" (include "opamp-fleet-server.fullname" .)) -}}
{{- end -}}

{{- define "opamp-fleet-server.basicAuthSecretName" -}}
{{- .Values.auth.basicAuth.existingSecret | default (printf "%s-basic-auth" (include "opamp-fleet-server.fullname" .)) -}}
{{- end -}}
