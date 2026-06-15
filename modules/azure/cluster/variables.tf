variable "name" {
  description = "AKS cluster name."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group for the cluster."
  type        = string
}

variable "k8s_version" {
  description = "Kubernetes version for the cluster."
  type        = string
  default     = "1.30"
}

variable "vnet_subnet_id" {
  description = "Resolved node subnet ID (from network-resolver) — nodes have no public IPs."
  type        = string
}

variable "key_vault_id" {
  description = "Resolved Key Vault ID (from kms module) holding the disk-encryption key."
  type        = string
}

variable "key_id" {
  description = "Resolved Key Vault Key ID (from kms module) for the disk encryption set."
  type        = string
}

variable "log_analytics_workspace_id" {
  description = "Log Analytics workspace ID for control-plane diagnostic settings (audit-grade logging)."
  type        = string
}

variable "node_vm_size" {
  description = "Node pool VM size."
  type        = string
  default     = "Standard_D4s_v5"
}

variable "node_min_count" {
  description = "Minimum node count (autoscale)."
  type        = number
  default     = 2
}

variable "node_max_count" {
  description = "Maximum node count (autoscale)."
  type        = number
  default     = 5
}

# Pod IPs come from this overlay CIDR (Azure CNI Overlay), NOT from the VNet, so
# the node subnet only sizes the node count and a small VNet can still run
# thousands of pods. 100.64.0.0/16 (CGNAT) gives ~65k pod IPs.
variable "pod_cidr" {
  description = "Overlay CIDR pods draw IPs from (Azure CNI Overlay; not VNet IPs). 100.64.0.0/16 (~65k pod IPs)."
  type        = string
  default     = "100.64.0.0/16"
}

# Virtual ClusterIP range — never real VNet IPs. Must not overlap the VNet CIDR
# or the pod CIDR.
variable "service_cidr" {
  description = "Virtual Service ClusterIP CIDR. Must not overlap the VNet or pod CIDR."
  type        = string
  default     = "172.20.0.0/16"
}

variable "dns_service_ip" {
  description = "Cluster DNS service IP — must fall inside service_cidr. Defaults to 172.20.0.10."
  type        = string
  default     = "172.20.0.10"
}

variable "max_pods" {
  description = "Max pods per node. node_max_count * max_pods bounds total pods; the overlay (pod_cidr) must be large enough. Default 250 (Overlay default)."
  type        = number
  default     = 250
}

# Private by default (no public API endpoint). For testing from outside the VNet,
# flip to false AND set api_server_authorized_ip_ranges to your /32 at apply time
# (-var) — never commit the IP. AKS rejects authorized IP ranges on a private
# cluster, so the access profile is only emitted when this is false.
variable "private_cluster_enabled" {
  description = "Private AKS API server (no public endpoint). Default true (hardened). Set false only for out-of-VNet testing, paired with api_server_authorized_ip_ranges."
  type        = bool
  default     = true
}

variable "api_server_authorized_ip_ranges" {
  description = "CIDRs allowed to reach the API server when private_cluster_enabled = false (e.g. [\"203.0.113.4/32\"]). Pass only at apply via -var; never commit an IP. Ignored when the cluster is private."
  type        = list(string)
  default     = []
}

# Entra-only by default (local Kubernetes accounts disabled). When true the
# cluster's kube_config cert/key are EMPTY, so the resolver/provider must use exec
# (kubelogin) auth; the cluster-resolver defaults auth_mode = "exec" to match. Set
# false only as a documented compromise (e.g. a CI runner that cannot run
# kubelogin), and then set the resolver auth_mode = "client_cert".
variable "local_account_disabled" {
  description = "Disable local Kubernetes accounts (Entra-only). Default true (hardened). When true, kube_config cert/key are empty — use exec (kubelogin) auth in the resolver/providers. Requires Entra integration (enabled automatically when this is true)."
  type        = bool
  default     = true
}

variable "admin_group_object_ids" {
  description = "Entra group object IDs granted cluster-admin via Azure RBAC for Kubernetes (only when local_account_disabled = true). Empty means authorization is via Azure RBAC role assignments only."
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Tags applied to the cluster resources."
  type        = map(string)
  default     = {}
}
