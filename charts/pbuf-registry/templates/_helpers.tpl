{{/*
Expand the name of the chart.
*/}}
{{- define "pbuf-registry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pbuf-registry.fullname" -}}
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
{{- define "pbuf-registry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pbuf-registry.labels" -}}
helm.sh/chart: {{ include "pbuf-registry.chart" . }}
{{ include "pbuf-registry.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pbuf-registry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pbuf-registry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pbuf-registry.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pbuf-registry.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Generate PostgreSQL DSN for internal database
*/}}
{{- define "pbuf-registry.postgresql.dsn" -}}
{{- $user := urlquery .Values.postgresql.auth.username | replace "+" "%20" -}}
{{- $pass := urlquery .Values.postgresql.auth.password | replace "+" "%20" -}}
{{- $db := urlquery .Values.postgresql.auth.database | replace "+" "%20" -}}
{{- printf "postgres://%s:%s@%s-postgresql:5432/%s?sslmode=disable" $user $pass (include "pbuf-registry.fullname" .) $db }}
{{- end }}


{{/*
Environment variables helper
*/}}
{{- define "pbuf-registry.env" -}}
- name: DATA_DATABASE_DSN
  valueFrom:
    secretKeyRef:
      name: {{ include "pbuf-registry.fullname" . }}
      key: DATA_DATABASE_DSN
- name: SERVER_STATIC_TOKEN
  valueFrom:
    secretKeyRef:
      name: {{ include "pbuf-registry.fullname" . }}
      key: SERVER_STATIC_TOKEN
      optional: true
- name: SERVER_GRPC_TLS_ENABLED
  value: "{{ .Values.service.grpc.tls.enabled }}"
- name: SERVER_GRPC_TLS_CERTFILE
  value: /app/certs/server-cert.pem
- name: SERVER_GRPC_TLS_KEYFILE
  value: /app/certs/server-key.pem
- name: SERVER_GRPC_AUTH_ENABLED
  value: "{{ .Values.service.grpc.auth.enabled }}"
- name: SERVER_GRPC_AUTH_TYPE
  value: "{{ .Values.service.grpc.auth.type }}"
{{- if .Values.service.grpc.auth.oidc }}
{{- if .Values.service.grpc.auth.oidc.issuerUrl }}
- name: SERVER_GRPC_AUTH_OIDC_ISSUERURL
  value: "{{ .Values.service.grpc.auth.oidc.issuerUrl }}"
{{- end }}
{{- if .Values.service.grpc.auth.oidc.clientId }}
- name: SERVER_GRPC_AUTH_OIDC_CLIENTID
  value: "{{ .Values.service.grpc.auth.oidc.clientId }}"
{{- end }}
{{- if .Values.service.grpc.auth.oidc.jwksUrl }}
- name: SERVER_GRPC_AUTH_OIDC_JWKSURL
  value: "{{ .Values.service.grpc.auth.oidc.jwksUrl }}"
{{- end }}
{{- if .Values.service.grpc.auth.oidc.usernameClaim }}
- name: SERVER_GRPC_AUTH_OIDC_USERNAMECLAIM
  value: "{{ .Values.service.grpc.auth.oidc.usernameClaim }}"
{{- end }}
{{- if .Values.service.grpc.auth.oidc.groupsClaim }}
- name: SERVER_GRPC_AUTH_OIDC_GROUPSCLAIM
  value: "{{ .Values.service.grpc.auth.oidc.groupsClaim }}"
{{- end }}
{{- if .Values.service.grpc.auth.oidc.adminGroups }}
- name: SERVER_GRPC_AUTH_OIDC_ADMINGROUPS
  {{- if kindIs "slice" .Values.service.grpc.auth.oidc.adminGroups }}
  value: {{ join "," .Values.service.grpc.auth.oidc.adminGroups | quote }}
  {{- else }}
  value: {{ .Values.service.grpc.auth.oidc.adminGroups | toString | quote }}
  {{- end }}
{{- end }}
- name: SERVER_GRPC_AUTH_OIDC_AUTOPROVISION
  value: "{{ .Values.service.grpc.auth.oidc.autoProvision }}"
{{- end }}
- name: SERVER_HTTP_AUTH_ENABLED
  value: "{{ .Values.service.http.auth.enabled }}"
- name: SERVER_HTTP_AUTH_TYPE
  value: "{{ .Values.service.http.auth.type }}"
{{- if .Values.service.http.auth.oidc }}
{{- if .Values.service.http.auth.oidc.issuerUrl }}
- name: SERVER_HTTP_AUTH_OIDC_ISSUERURL
  value: "{{ .Values.service.http.auth.oidc.issuerUrl }}"
{{- end }}
{{- if .Values.service.http.auth.oidc.clientId }}
- name: SERVER_HTTP_AUTH_OIDC_CLIENTID
  value: "{{ .Values.service.http.auth.oidc.clientId }}"
{{- end }}
{{- if .Values.service.http.auth.oidc.jwksUrl }}
- name: SERVER_HTTP_AUTH_OIDC_JWKSURL
  value: "{{ .Values.service.http.auth.oidc.jwksUrl }}"
{{- end }}
{{- if .Values.service.http.auth.oidc.usernameClaim }}
- name: SERVER_HTTP_AUTH_OIDC_USERNAMECLAIM
  value: "{{ .Values.service.http.auth.oidc.usernameClaim }}"
{{- end }}
{{- if .Values.service.http.auth.oidc.groupsClaim }}
- name: SERVER_HTTP_AUTH_OIDC_GROUPSCLAIM
  value: "{{ .Values.service.http.auth.oidc.groupsClaim }}"
{{- end }}
{{- if .Values.service.http.auth.oidc.adminGroups }}
- name: SERVER_HTTP_AUTH_OIDC_ADMINGROUPS
  {{- if kindIs "slice" .Values.service.http.auth.oidc.adminGroups }}
  value: {{ join "," .Values.service.http.auth.oidc.adminGroups | quote }}
  {{- else }}
  value: {{ .Values.service.http.auth.oidc.adminGroups | toString | quote }}
  {{- end }}
{{- end }}
- name: SERVER_HTTP_AUTH_OIDC_AUTOPROVISION
  value: "{{ .Values.service.http.auth.oidc.autoProvision }}"
{{- end }}
- name: DAEMONS_PROTOPARSING_WEBHOOK_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "pbuf-registry.fullname" . }}
      key: DAEMONS_PROTOPARSING_WEBHOOK_URL
      optional: true
{{- if .Values.background.protoparser.webhook }}
{{- if .Values.background.protoparser.webhook.minSeverity }}
- name: DAEMONS_PROTOPARSING_WEBHOOK_MIN_SEVERITY
  value: "{{ .Values.background.protoparser.webhook.minSeverity }}"
{{- end }}
{{- if .Values.background.protoparser.webhook.format }}
- name: DAEMONS_PROTOPARSING_WEBHOOK_FORMAT
  value: "{{ .Values.background.protoparser.webhook.format }}"
{{- end }}
{{- if .Values.background.protoparser.webhook.timeout }}
- name: DAEMONS_PROTOPARSING_WEBHOOK_TIMEOUT
  value: "{{ .Values.background.protoparser.webhook.timeout }}"
{{- end }}
{{- end }}
{{- end }}

{{/*
Volume mounts
*/}}
{{- define "pbuf-registry.volumeMounts" -}}
{{- if .Values.service.grpc.tls.enabled }}
- mountPath: /app/certs/server-cert.pem
  name: secret
  readOnly: true
  subPath: server-cert.pem
- mountPath: /app/certs/server-key.pem
  name: secret
  readOnly: true
  subPath: server-key.pem
{{- end }}
{{- end }}

{{/*
Database migrations initContainer shared by server and background workers
*/}}
{{- define "pbuf-registry.migrationsInitContainer" -}}
- name: migrations
  securityContext:
    {{- toYaml .Values.securityContext | nindent 4 }}
  image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  command:
    - /app/pbuf-migrations
  env:
    {{- include "pbuf-registry.env" . | nindent 4 }}
  volumeMounts:
    {{- include "pbuf-registry.volumeMounts" . | nindent 4 }}
{{- end }}