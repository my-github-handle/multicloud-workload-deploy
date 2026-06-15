variable "name" {
  description = "Name prefix for all network resources."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group the network resources are created in."
  type        = string
}

variable "address_space" {
  description = "VNet address space."
  type        = list(string)
  default     = ["10.0.0.0/16"]
}

variable "node_subnet_prefix" {
  description = "Address prefix for the AKS node subnet (no public IPs; egress forced through the firewall via UDR)."
  type        = string
  default     = "10.0.0.0/20"
}

variable "firewall_subnet_prefix" {
  description = "Address prefix for AzureFirewallSubnet (must be named exactly AzureFirewallSubnet, >= /26)."
  type        = string
  default     = "10.0.64.0/26"
}

variable "egress_allowed_fqdns" {
  description = "FQDNs allowed for egress through the Azure Firewall application rule collection (control-plane FQDN, ghcr.io, Azure API endpoints, observability sinks). Everything else is default-deny via the UDR + firewall."
  type        = list(string)
  default     = ["ghcr.io", "github.com", "*.azurecr.io", "login.microsoftonline.com"]
}

variable "allow_aks_egress" {
  description = "Add the AzureKubernetesService FQDN tag + AKS required network rules (NTP, API tunnel) to the firewall. Required when the cluster uses outbound_type = userDefinedRouting (egress forced through this firewall) — without it the cluster cannot bootstrap. Default true."
  type        = bool
  default     = true
}

variable "egress_allowed_cidrs" {
  description = "Destination CIDR blocks allowed for egress via the firewall network rule collection (Azure service tags not covered by FQDN rules)."
  type        = list(string)
  default     = []
}

variable "network_watcher_name" {
  description = "Name of the region's Network Watcher (Azure auto-creates NetworkWatcher_<region>). The flow log attaches to it rather than creating a second watcher."
  type        = string
  default     = "NetworkWatcher_eastus"
}

variable "network_watcher_resource_group" {
  description = "Resource group holding the region's Network Watcher (Azure's default is NetworkWatcherRG)."
  type        = string
  default     = "NetworkWatcherRG"
}

variable "flow_log_retention_days" {
  description = "Time-based immutability retention (days) on the customer-owned flow-logs Storage container. The always-on audit floor."
  type        = number
  default     = 365
}

variable "tags" {
  description = "Tags applied to all resources."
  type        = map(string)
  default     = {}
}
