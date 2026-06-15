variable "mode" {
  description = "\"provision\" feeds the network module's outputs straight through; \"byo\" looks up an existing VPC/subnets via data sources."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

# --- provision mode: outputs of modules/aws/network are fed in here ---
variable "provisioned_vpc_id" {
  description = "VPC ID from modules/aws/network (provision mode). Ignored in byo mode."
  type        = string
  default     = ""
}

variable "provisioned_subnet_ids" {
  description = "Node (private) subnet IDs from modules/aws/network (provision mode)."
  type        = list(string)
  default     = []
}

variable "provisioned_pod_subnet_ids" {
  description = "Pod subnet IDs from modules/aws/network (provision mode). Empty when the data plane uses node subnets for pods."
  type        = list(string)
  default     = []
}

variable "provisioned_egress_path_ref" {
  description = "Egress path ref (Network Firewall ARN) from modules/aws/network (provision mode)."
  type        = string
  default     = ""
}

# --- byo mode: locate the customer's existing VPC + subnets ---
variable "byo_vpc_id" {
  description = "Existing VPC ID to look up (byo mode)."
  type        = string
  default     = ""
}

variable "byo_subnet_tag_filter" {
  description = "Tag key=value used to select the existing private (node) subnets in byo mode (e.g. { \"kubernetes.io/role/internal-elb\" = \"1\" })."
  type        = map(string)
  default     = {}
}

variable "byo_pod_subnet_tag_filter" {
  description = "Tag key=value used to select the existing pod subnets in byo mode (e.g. { \"kubernetes.io/role/cni\" = \"1\" }). Empty selects no separate pod subnets (pods share the node subnets)."
  type        = map(string)
  default     = {}
}

variable "byo_egress_path_ref" {
  description = "Optional customer-supplied egress path reference (byo mode); empty when the customer owns the edge firewall. An empty value is the deliberate \"customer-owned edge\" signal that preflight treats as amber."
  type        = string
  default     = ""
}
