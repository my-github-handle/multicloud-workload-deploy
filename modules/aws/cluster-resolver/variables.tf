variable "mode" {
  description = "\"provision\" feeds the cluster module outputs through; \"byo\" looks up an existing EKS cluster via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "cluster_name" {
  description = "EKS cluster name. In provision mode the created cluster's name; in byo mode the existing cluster to look up. Used to fetch a fresh auth token in both modes."
  type        = string
}

# provision-mode passthrough (from modules/aws/cluster)
variable "provisioned_endpoint" {
  description = "Cluster endpoint from the cluster module (provision mode)."
  type        = string
  default     = ""
}

variable "provisioned_ca" {
  description = "Cluster CA data from the cluster module (provision mode)."
  type        = string
  default     = ""
}
