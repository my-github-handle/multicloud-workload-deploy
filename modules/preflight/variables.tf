variable "preflight_binary" {
  description = "Absolute path to the preflight checker binary (operator/cmd/preflight build output)."
  type        = string
}

variable "kubeconfig_path" {
  description = "Path to the kubeconfig the preflight binary uses for the real Kubernetes stages 4-5."
  type        = string
}

variable "namespace" {
  description = "Target workload namespace passed to the preflight binary (--namespace)."
  type        = string
}

variable "fail_on_red" {
  description = "When true, a red verdict fails the plan via the data-source postcondition. The check block always reports; this flag controls hard-blocking."
  type        = bool
  default     = true
}

variable "install_tier_override" {
  description = "Explicit install tier override: \"A\", \"B\", or \"\" (empty = derive from the preflight report's k8s.installtier result)."
  type        = string
  default     = ""
  validation {
    condition     = contains(["A", "B", ""], var.install_tier_override)
    error_message = "install_tier_override must be \"A\", \"B\", or \"\"."
  }
}

variable "mode" {
  description = "Preflight mode passed to the binary: \"agnostic\" (BYOC fast path) or \"full\" (greenfield; stages satisfied by provisioning are informational)."
  type        = string
  default     = "agnostic"
  validation {
    condition     = contains(["agnostic", "full"], var.mode)
    error_message = "mode must be \"agnostic\" or \"full\"."
  }
}

variable "cloud" {
  description = "Cloud provider for the binary's --cloud flag: \"\" (fake/agnostic), \"aws\", \"gcp\", or \"azure\". aws-full passes \"aws\" so the real AWS provider runs the cloud stages."
  type        = string
  default     = ""
  validation {
    condition     = contains(["", "aws", "gcp", "azure"], var.cloud)
    error_message = "cloud must be \"\", \"aws\", \"gcp\", or \"azure\"."
  }
}
