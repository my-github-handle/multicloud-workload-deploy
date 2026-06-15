variable "mode" {
  description = "\"provision\" feeds the network module's outputs straight through; \"byo\" looks up an existing VPC/subnet via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "project_id" {
  description = "GCP project ID (used by BYO data-source lookups)."
  type        = string
}

variable "region" {
  description = "Region of the subnet (BYO lookup)."
  type        = string
  default     = ""
}

# --- provision mode: outputs of modules/gcp/network are fed in here ---
variable "provisioned_network_self_link" {
  description = "Network self-link from modules/gcp/network (provision mode). Ignored in byo mode."
  type        = string
  default     = ""
}

variable "provisioned_subnet_self_links" {
  description = "Subnet self-links from modules/gcp/network (provision mode)."
  type        = list(string)
  default     = []
}

variable "provisioned_egress_path_ref" {
  description = "Egress path ref (firewall policy name) from modules/gcp/network (provision mode)."
  type        = string
  default     = ""
}

# --- byo mode: locate the customer's existing VPC + subnet ---
variable "byo_network_name" {
  description = "Existing VPC network name to look up (byo mode)."
  type        = string
  default     = ""
}

variable "byo_subnet_name" {
  description = "Existing subnet name to look up (byo mode)."
  type        = string
  default     = ""
}

variable "byo_egress_path_ref" {
  description = "Optional customer-supplied egress path reference (byo mode); empty when the customer owns the edge firewall."
  type        = string
  default     = ""
}
