{{/*
The namespace the controller watches and manages. The operator is namespace-scoped: it owns the
Workload resources and their child objects in exactly one namespace. That may differ from the
namespace the operator itself is installed into (Deployment/ServiceAccount/Service), so the Role
and RoleBinding that grant access to the managed resources must live in this target namespace,
not the install namespace. Defaults to the install namespace when watchNamespace is unset.
*/}}
{{- define "workload-operator.targetNamespace" -}}
{{- if .Values.watchNamespace -}}
{{- .Values.watchNamespace -}}
{{- else -}}
{{- .Values.namespace -}}
{{- end -}}
{{- end -}}
