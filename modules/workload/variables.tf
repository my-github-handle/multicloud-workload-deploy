variable "install_tier" {
  description = "\"A\" applies a Workload CR (operator reconciles). \"B\" renders charts/workload directly via helm_release."
  type        = string
  validation {
    condition     = contains(["A", "B"], var.install_tier)
    error_message = "install_tier must be \"A\" or \"B\"."
  }
}

variable "name" {
  description = "Workload name (CRD metadata.name / charts/workload .Values.name)."
  type        = string
}

variable "namespace" {
  description = "Target namespace."
  type        = string
}

variable "spec_yaml" {
  description = <<-EOT
    The Workload spec as YAML — the SINGLE source of the workload's shape for both tiers. Its
    fields match the Workload CRD spec and charts/workload values.schema.json (minus name/namespace,
    which are identity/wiring): image, port, autoscale{minReplicas,maxReplicas,targetCPUUtilization},
    and optionally livenessProbe{path,port}, readinessProbe{path,port}, resources,
    securityContext, podSecurityContext, rolloutStrategy, ingressClass, ingress.

    Tier A wraps this verbatim as the Workload CR's `spec`; Tier B passes it as helm values
    (merged with name/namespace/pdb). One document, so the CR spec and the chart values cannot
    drift. Per-cloud values are supplied by composing/merging YAML at the root.
  EOT
  type        = string
  validation {
    condition     = can(yamldecode(var.spec_yaml))
    error_message = "spec_yaml must be valid YAML."
  }
  validation {
    condition     = alltrue([for k in ["image", "port", "autoscale"] : contains(keys(yamldecode(var.spec_yaml)), k)])
    error_message = "spec_yaml must include at least image, port, and autoscale."
  }
}

variable "workload_chart_path" {
  description = "Path to the shared workload Helm chart (charts/workload). Tier B only."
  type        = string
  default     = "../../charts/workload"
}

variable "pdb_min_available" {
  description = "PodDisruptionBudget minAvailable (charts/workload .Values.pdb.minAvailable). Tier B only (the chart owns the PDB; the CRD does not expose it)."
  type        = number
  default     = 1
}

variable "wait_for_ready" {
  description = "Tier A only: block the apply until the operator sets the Workload Ready=True. Disable on rate-limited/slow clusters where the readiness poll flakes; readiness can then be confirmed out-of-band (kubectl wait)."
  type        = bool
  default     = true
}

variable "wait_timeout" {
  description = "Tier A only: timeout for the create/readiness wait on the Workload CR (e.g. \"5m\")."
  type        = string
  default     = "5m"
}
