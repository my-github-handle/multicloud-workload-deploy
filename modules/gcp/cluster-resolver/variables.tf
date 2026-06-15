variable "mode" {
  description = "\"provision\" feeds the cluster module outputs through; \"byo\" looks up an existing GKE cluster via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "project_id" {
  description = "GCP project ID (BYO lookup)."
  type        = string
}

variable "location" {
  description = "Cluster location (region/zone). In provision mode the created cluster's location; in byo mode the existing cluster's location."
  type        = string
}

variable "cluster_name" {
  description = "GKE cluster name. In provision mode the created cluster's name; in byo mode the existing cluster to look up."
  type        = string
}

# provision-mode passthrough (from modules/gcp/cluster)
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
