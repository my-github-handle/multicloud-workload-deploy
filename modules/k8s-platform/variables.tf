variable "install_tier" {
  description = "\"A\" installs the workload-operator chart (CRD + namespace-scoped RBAC + controller). \"B\" is a no-op (workload module renders charts/workload directly)."
  type        = string
  validation {
    condition     = contains(["A", "B"], var.install_tier)
    error_message = "install_tier must be \"A\" or \"B\"."
  }
}

variable "namespace" {
  description = "Namespace the operator is installed into and (in Tier A) watches."
  type        = string
}

variable "operator_chart_path" {
  description = "Path to the workload-operator Helm chart (charts/workload-operator)."
  type        = string
  default     = "../../charts/workload-operator"
}

variable "operator_image_repository" {
  description = "Operator controller image repository."
  type        = string
  default     = "ghcr.io/ops-dev/workload-operator"
}

variable "operator_image_tag" {
  description = "Operator controller image tag."
  type        = string
  default     = "0.1.0"
}

variable "create_namespace" {
  description = "Whether the operator chart release should create the namespace."
  type        = bool
  default     = true
}
