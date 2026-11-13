{{- define "saas-product.labels" -}}
app.kubernetes.io/name: saas-product
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}
{{- define "saas-product.selectorLabels" -}}
app.kubernetes.io/name: saas-product
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
{{- define "saas-product.planName" -}}
migration-plan-{{ .Values.migration.plan | sha256sum | trunc 40 }}
{{- end -}}
