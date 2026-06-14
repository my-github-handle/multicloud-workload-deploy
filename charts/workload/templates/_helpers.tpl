{{/*
Common labels stamped on every child object so a workload's resources can be identified and
governed as one unit (kubectl/Prometheus/GitOps selectors, fleet ownership queries).
  - name / instance identify the specific workload
  - part-of groups all of a workload's objects under one application
  - managed-by marks the operator as the governing controller
*/}}
{{- define "workload.labels" -}}
app.kubernetes.io/name: {{ .Values.name }}
app.kubernetes.io/instance: {{ .Values.name }}
app.kubernetes.io/part-of: {{ .Values.name }}
app.kubernetes.io/managed-by: workload-operator
{{- end -}}

{{/*
Selector labels are the stable subset used in label selectors (Deployment/Service/PDB/HPA
match). These must never change for an existing workload — they are the immutable identity key —
so they are intentionally narrower than the full label set above.
*/}}
{{- define "workload.selectorLabels" -}}
app.kubernetes.io/name: {{ .Values.name }}
app.kubernetes.io/instance: {{ .Values.name }}
{{- end -}}
