variable "mode" {
  description = "\"provision\" feeds the network module's outputs straight through; \"byo\" looks up an existing VNet/subnets via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

# --- provision mode: outputs of modules/azure/network are fed in here ---
variable "provisioned_vnet_id" {
  description = "VNet ID from modules/azure/network (provision mode). Ignored in byo mode."
  type        = string
  default     = ""
}

variable "provisioned_subnet_ids" {
  description = "Node subnet IDs from modules/azure/network (provision mode)."
  type        = list(string)
  default     = []
}

variable "provisioned_egress_path_ref" {
  description = "Egress path ref (Azure Firewall ID) from modules/azure/network (provision mode)."
  type        = string
  default     = ""
}

# --- byo mode: locate the customer's existing VNet + subnets ---
variable "byo_resource_group_name" {
  description = "Resource group of the existing VNet (byo mode)."
  type        = string
  default     = ""
}

variable "byo_vnet_name" {
  description = "Existing VNet name to look up (byo mode)."
  type        = string
  default     = ""
}

variable "byo_subnet_names" {
  description = "Existing subnet names to resolve within the BYO VNet (byo mode)."
  type        = list(string)
  default     = []
}

variable "byo_egress_path_ref" {
  description = "Optional customer-supplied egress path reference (byo mode); empty when the customer owns the edge firewall."
  type        = string
  default     = ""
}
