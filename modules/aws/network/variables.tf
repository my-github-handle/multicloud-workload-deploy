variable "name" {
  description = "Name prefix for all network resources."
  type        = string
}

variable "azs" {
  description = "Availability zones to spread subnets across. At least two for HA — every NAT gateway, firewall endpoint, and data-plane route table is provisioned per-AZ so the loss of one AZ never strands another's egress."
  type        = list(string)
  validation {
    condition     = length(var.azs) >= 2
    error_message = "Provide at least two availability zones for an HA topology."
  }
}

# --- Primary CIDR: edge / control path (public + firewall-endpoint subnets) ---
variable "vpc_primary_cidr" {
  description = "Primary VPC CIDR. Carries only the edge tiers — public subnets (NAT gateways, load balancers) and the dedicated Network Firewall endpoint subnets. Kept small and stable; it never grows with the workload."
  type        = string
  default     = "10.0.0.0/24"
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDRs (primary CIDR), one per AZ — NAT gateways and load balancers."
  type        = list(string)
  default     = ["10.0.0.0/27", "10.0.0.32/27", "10.0.0.64/27"]
}

variable "firewall_subnet_cidrs" {
  description = "Dedicated AWS Network Firewall endpoint subnet CIDRs (primary CIDR), one per AZ. AWS requires the firewall endpoint to live in its own subnet — placing it in a node subnet creates a routing loop. Small /28s suffice (one ENI per AZ)."
  type        = list(string)
  default     = ["10.0.0.128/28", "10.0.0.144/28", "10.0.0.160/28"]
}

# --- Secondary CIDR: data plane (node + pod subnets) ---
variable "vpc_secondary_cidr" {
  description = "Secondary VPC CIDR for the data plane (node and pod subnets). A large CGNAT-range block so nodes and Cilium-managed pods never exhaust the routable primary CIDR. Associated as a secondary IPv4 block on the VPC."
  type        = string
  default     = "100.64.0.0/16"
}

variable "node_subnet_cidrs" {
  description = "Node subnet CIDRs (secondary CIDR), one per AZ — private, no public IPs. Default route is forced through the same-AZ Network Firewall endpoint."
  type        = list(string)
  default     = ["100.64.0.0/18", "100.64.64.0/18", "100.64.128.0/18"]
}

variable "pod_subnet_cidrs" {
  description = "Pod subnet CIDRs (secondary CIDR), one per AZ. The VPC CNI in custom-networking mode allocates pod IPs from these subnets (discovered via the kubernetes.io/role/cni tag), isolating pod IP churn from the node subnets. Egress is forced through the same-AZ firewall endpoint, identical to the node subnets."
  type        = list(string)
  default     = ["100.64.192.0/19", "100.64.224.0/19", "100.65.0.0/19"]
}

variable "pod_subnet_tags" {
  description = "Tags applied to the pod subnets so the CNI discovers them. The default tags the subnets for Cilium ENI-mode allocation (kubernetes.io/role/cni)."
  type        = map(string)
  default     = { "kubernetes.io/role/cni" = "1" }
}

variable "egress_allowed_fqdns" {
  description = "FQDNs allowed for egress through the Network Firewall (control-plane FQDN, ghcr.io, AWS API endpoints, observability sinks). Everything else is default-deny."
  type        = list(string)
  default     = ["ghcr.io", "github.com"]
}

variable "egress_allowed_cidrs" {
  description = "CIDR blocks allowed for egress (e.g. AWS service prefixes not covered by FQDN rules)."
  type        = list(string)
  default     = []
}

variable "flow_log_retention_days" {
  description = "Object-lock retention (days) on the customer-owned VPC Flow Logs S3 bucket. The always-on audit floor."
  type        = number
  default     = 365
}

variable "tags" {
  description = "Tags applied to all resources."
  type        = map(string)
  default     = {}
}
