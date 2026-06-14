# --- Cluster access (the only credential) ---
variable "kubeconfig_path" {
  description = "Path to the kubeconfig granting access to the EXISTING cluster."
  type        = string
}

variable "kube_context" {
  description = "Optional kubeconfig context name. Empty uses the current-context."
  type        = string
  default     = ""
}

# --- Preflight ---
variable "preflight_binary" {
  description = "Absolute path to the preflight binary (operator/cmd/preflight build output)."
  type        = string
}

variable "fail_on_red" {
  description = "Block apply when the preflight verdict is red."
  type        = bool
  default     = true
}

variable "install_tier_override" {
  description = "Force the install tier (\"A\"|\"B\") instead of deriving it from the preflight report. Empty = derive."
  type        = string
  default     = ""
  validation {
    condition     = contains(["A", "B", ""], var.install_tier_override)
    error_message = "install_tier_override must be \"A\", \"B\", or \"\" (empty = derive)."
  }
}

# --- Namespace / platform ---
variable "namespace" {
  description = "Workload namespace."
  type        = string
  default     = "workload-system"
}

variable "operator_image_repository" {
  description = "Operator image repository."
  type        = string
  default     = "ghcr.io/ops-dev/workload-operator"
}

variable "operator_image_tag" {
  description = "Operator image tag."
  type        = string
  default     = "0.1.0"
}

# --- Security ---
# No control_plane_fqdn here: the in-cluster NetworkPolicy floor is CIDR/port-based and cannot do
# FQDN-granular egress. FQDN scoping is the perimeter firewall's job / Cilium toFQDNs. Only the
# control-plane PORT is parameterized for the in-cluster egress allow.
variable "control_plane_port" {
  description = "Control-plane egress port (the in-cluster egress-allow opens this port on a wide CIDR minus the metadata IPs)."
  type        = number
  default     = 443
}

variable "psa_enforce_level" {
  description = "Pod Security Admission enforce level for the workload namespace: \"restricted\" (default, secure floor), \"baseline\" (permits root images — required if the workload runs as root), or \"privileged\". audit/warn always track restricted."
  type        = string
  default     = "restricted"
  validation {
    condition     = contains(["restricted", "baseline", "privileged"], var.psa_enforce_level)
    error_message = "psa_enforce_level must be \"restricted\", \"baseline\", or \"privileged\"."
  }
}

# --- Observability ---
variable "observability_enabled" {
  description = "Apply the in-cluster ServiceMonitor + dashboard. Disable if the Prometheus-operator CRD is absent."
  type        = bool
  default     = true
}

# --- Workload ---
variable "workload_name" {
  description = "Workload name (CR metadata.name / chart .Values.name)."
  type        = string
}

variable "workload_spec_yaml" {
  description = <<-EOT
    The Workload spec as YAML — the single source of the workload's shape, fed to both install
    tiers. Fields match the Workload CRD spec and charts/workload values.schema.json (minus
    name/namespace): image, port, autoscale{minReplicas,maxReplicas,targetCPUUtilization}, and
    optionally livenessProbe, readinessProbe, resources, securityContext, podSecurityContext,
    rolloutStrategy, ingressClass, ingress. Provide a file with `file("workload.yaml")` or inline
    heredoc. Per-cloud values are merged into this YAML at the call site.
  EOT
  type        = string
}

variable "workload_wait_for_ready" {
  description = "Tier A only: block the apply until the operator sets Ready=True. Disable on rate-limited/slow clusters; confirm readiness out-of-band instead."
  type        = bool
  default     = true
}
