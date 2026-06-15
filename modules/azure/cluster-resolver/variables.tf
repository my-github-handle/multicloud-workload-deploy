variable "mode" {
  description = "\"provision\" feeds the cluster module outputs through; \"byo\" looks up an existing AKS cluster via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "resource_group_name" {
  description = "Resource group of the cluster (byo mode lookup)."
  type        = string
  default     = ""
}

variable "provided_cluster_name" {
  description = "Existing AKS cluster name to look up (byo mode)."
  type        = string
  default     = ""
}

# Which auth form the resolver emits. "exec" is the Entra/kubelogin exec-plugin
# form, REQUIRED when local_account_disabled = true (the kube_config cert/key are
# then empty). "client_cert" is the local-account cert/key pair. The resolver
# emits a tagged `auth` object carrying `mode` plus the fields the chosen form
# needs; the root's providers.tf branches on `auth.mode`.
variable "auth_mode" {
  description = "\"client_cert\" (local-account kube_config cert/key) or \"exec\" (Entra kubelogin exec plugin). MUST be \"exec\" for hardened clusters with local_account_disabled = true."
  type        = string
  default     = "exec"
  validation {
    condition     = contains(["client_cert", "exec"], var.auth_mode)
    error_message = "auth_mode must be \"client_cert\" or \"exec\"."
  }
}

variable "cluster_name_for_exec" {
  description = "AKS cluster name passed to the kubelogin exec plugin / az aks get-credentials (exec auth_mode)."
  type        = string
  default     = ""
}

# provision-mode passthrough (from modules/azure/cluster)
variable "provisioned_host" {
  description = "Cluster API host from the cluster module (provision mode)."
  type        = string
  default     = ""
}

variable "provisioned_ca" {
  description = "Cluster CA data from the cluster module (provision mode)."
  type        = string
  default     = ""
}

variable "provisioned_client_certificate" {
  description = "Client certificate from the cluster kube_config (provision mode, client_cert only). Empty when local_account_disabled = true."
  type        = string
  default     = ""
}

variable "provisioned_client_key" {
  description = "Client key from the cluster kube_config (provision mode, client_cert only). Empty when local_account_disabled = true."
  type        = string
  default     = ""
}
