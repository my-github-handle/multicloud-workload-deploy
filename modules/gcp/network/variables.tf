variable "name" {
  description = "Name prefix for all network resources."
  type        = string
}

variable "project_id" {
  description = "GCP project ID the network lives in."
  type        = string
}

variable "project_number" {
  description = "GCP project number (from the project module). Used to name the flow-log bucket without re-reading the project, so the module does not depend on the project existing at plan time."
  type        = string
}

variable "region" {
  description = "GCP region for the subnet, router, and NAT."
  type        = string
  default     = "us-central1"
}

# --- Data-plane CIDR plan ---
# Nodes, pods, and services all draw from the CGNAT block 100.64.0.0/16, with
# pods on the largest sub-block so pod IP churn never exhausts routable space.
variable "subnet_cidr" {
  description = "Primary CIDR of the node subnet (no public IPs on nodes)."
  type        = string
  default     = "100.64.0.0/18"
}

variable "pods_cidr" {
  description = "Secondary range CIDR for GKE pods (alias IPs). The largest data-plane sub-block."
  type        = string
  default     = "100.64.128.0/17"
}

variable "services_cidr" {
  description = "Secondary range CIDR for GKE services (alias IPs). A small data-plane sub-block."
  type        = string
  default     = "100.64.64.0/19"
}

variable "egress_allowed_fqdns" {
  description = "FQDNs allowed for egress through the VPC firewall policy (control-plane FQDN, ghcr.io, Google API endpoints, observability sinks). Everything else is default-deny."
  type        = list(string)
  default     = ["ghcr.io", "github.com"]
}

variable "egress_allowed_cidrs" {
  description = "CIDR blocks allowed for egress (e.g. Google API ranges not covered by FQDN rules)."
  type        = list(string)
  default     = []
}

variable "google_api_cidrs" {
  description = "Private Google Access VIP range(s). Under default-deny egress these MUST be allowed or nodes cannot reach Artifact Registry/KMS/Secret Manager/Workload-Identity and will not register. Default is the RESTRICTED VIP 199.36.153.4/30 (use restricted.googleapis.com). The speculative anycast 34.126.0.0/18 is deliberately DROPPED; add the private VIP 199.36.153.8/30 only if not using the restricted VIP for all Google APIs."
  type        = list(string)
  default     = ["199.36.153.4/30"]
}

variable "master_ipv4_cidr_block" {
  description = "The /28 CIDR of the GKE private control-plane endpoint. Under default-deny egress this MUST be allowed or nodes cannot reach the control plane and the cluster bricks. Must match the value passed to the cluster module."
  type        = string
  default     = "172.16.0.0/28"
}

variable "intra_vpc_cidrs" {
  description = "Intra-VPC ranges (subnet + pod + service CIDRs) that pod-to-pod / pod-to-node / pod-to-service traffic needs under default-deny egress. Defaults to the three CIDRs this module manages; override if BYO ranges differ."
  type        = list(string)
  default     = ["100.64.0.0/18", "100.64.128.0/17", "100.64.64.0/19"]
}

variable "flow_log_retention_days" {
  description = "Bucket-lock retention (days) on the customer-owned VPC Flow Logs GCS bucket. The always-on audit floor."
  type        = number
  default     = 365
}

variable "labels" {
  description = "Labels applied to all resources that support them."
  type        = map(string)
  default     = {}
}
