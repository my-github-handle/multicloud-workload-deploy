variable "name" {
  description = "EKS cluster name."
  type        = string
}

variable "k8s_version" {
  description = "Kubernetes version for the cluster."
  type        = string
  default     = "1.34"
}

variable "private_subnet_ids" {
  description = "Resolved node subnet IDs (from network-resolver) — nodes have no public IPs. Spread across AZs for HA."
  type        = list(string)
}

variable "pod_subnet_ids" {
  description = "Resolved pod subnet IDs (from network-resolver), one per AZ in the secondary CIDR. VPC CNI custom networking places pod secondary ENIs here via a per-AZ ENIConfig, keeping pods off the node subnet's address space. Empty disables custom networking (pods share the node subnets)."
  type        = list(string)
  default     = []
}

variable "node_azs" {
  description = "Availability zones aligned positionally with pod_subnet_ids (index i of pod_subnet_ids is in node_azs[i]). Used to build the per-AZ ENIConfig map keyed by the topology.kubernetes.io/zone label. Required when pod_subnet_ids is set."
  type        = list(string)
  default     = []
  validation {
    condition     = length(var.pod_subnet_ids) == 0 || length(var.node_azs) == length(var.pod_subnet_ids)
    error_message = "node_azs must have one entry per pod_subnet_ids entry (same order)."
  }
}

variable "vpc_cni_addon_version" {
  description = "Pinned vpc-cni EKS addon version (empty = the cluster's default version)."
  type        = string
  default     = ""
}

variable "coredns_addon_version" {
  description = "Pinned coredns EKS addon version (empty = default)."
  type        = string
  default     = ""
}

variable "kube_proxy_addon_version" {
  description = "Pinned kube-proxy EKS addon version (empty = default)."
  type        = string
  default     = ""
}

variable "kms_key_arn" {
  description = "Resolved CMK ARN (from the kms module) for EKS secrets envelope encryption at rest."
  type        = string
}

variable "service_ipv4_cidr" {
  description = "Virtual ClusterIP Service CIDR. Must NOT overlap either VPC CIDR — Service IPs are never real VPC IPs."
  type        = string
  default     = "172.20.0.0/16"
}

variable "endpoint_public_access" {
  description = "Expose the EKS API endpoint publicly. Default false — a private cluster reachable only from within the VPC."
  type        = bool
  default     = false
}

variable "public_access_cidrs" {
  description = "CIDRs allowed to reach the public API endpoint when endpoint_public_access is true. Ignored when the endpoint is private."
  type        = list(string)
  default     = []
}

variable "enabled_cluster_log_types" {
  description = "Control-plane log types streamed to CloudWatch (audit-grade)."
  type        = list(string)
  default     = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
}

variable "node_instance_types" {
  description = "Managed node group instance types."
  type        = list(string)
  default     = ["m6i.large"]
}

variable "node_min_size" {
  description = "Minimum managed node group size."
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Maximum managed node group size."
  type        = number
  default     = 5
}

variable "node_desired_size" {
  description = "Desired managed node group size."
  type        = number
  default     = 2
}

variable "tags" {
  description = "Tags applied to the cluster resources."
  type        = map(string)
  default     = {}
}
