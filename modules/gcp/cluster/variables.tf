variable "name" {
  description = "GKE cluster name."
  type        = string
}

variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "project_number" {
  description = "GCP project number (from the project module). Used to construct the GKE service-agent member without re-reading the project."
  type        = string
}

variable "region" {
  description = "GKE cluster location (region for a regional cluster)."
  type        = string
  default     = "us-central1"
}

variable "k8s_version" {
  description = "Kubernetes minimum master version / node version (or leave to the release channel)."
  type        = string
  default     = "1.30"
}

variable "release_channel" {
  description = "GKE release channel: RAPID | REGULAR | STABLE."
  type        = string
  default     = "REGULAR"
}

variable "network_self_link" {
  description = "Resolved VPC network self-link (from network-resolver)."
  type        = string
}

variable "subnet_self_link" {
  description = "Resolved node subnet self-link (from network-resolver / network module)."
  type        = string
}

variable "pods_range_name" {
  description = "Secondary range name for pods (alias IPs)."
  type        = string
}

variable "services_range_name" {
  description = "Secondary range name for services (alias IPs)."
  type        = string
}

variable "kms_key_id" {
  description = "Resolved CryptoKey id (from kms module) for GKE database/application-layer secrets encryption at rest."
  type        = string
}

variable "master_ipv4_cidr_block" {
  description = "The /28 CIDR for the private control-plane endpoint."
  type        = string
  default     = "172.16.0.0/28"
}

variable "node_machine_type" {
  description = "Node pool machine type."
  type        = string
  default     = "e2-standard-4"
}

variable "node_min_count" {
  description = "Minimum nodes per zone."
  type        = number
  default     = 1
}

variable "node_max_count" {
  description = "Maximum nodes per zone."
  type        = number
  default     = 3
}

variable "enable_private_endpoint" {
  description = "When true (default), the control-plane endpoint is private (in-VPC only). Set false ONLY for testing to reach the API server from outside the VPC; pair with master_authorized_networks to restrict access to a CIDR allowlist."
  type        = bool
  default     = true
}

variable "master_authorized_networks" {
  description = "CIDR blocks allowed to reach the control-plane endpoint, as a list of { cidr_block, display_name }. Empty by default. When enable_private_endpoint=false (testing), set this to a tight allowlist so the public endpoint is not open to the world. Supply the CIDR at apply time; do not commit it."
  type = list(object({
    cidr_block   = string
    display_name = string
  }))
  default = []
}

variable "enable_secret_manager_csi_addon" {
  description = "Enable the GKE-managed Secret Manager CSI driver add-on. The secrets module's SecretProviderClass apply FAILS on a clean greenfield cluster unless the Secrets-Store-CSI driver + GCP provider are present. Enabling this managed add-on installs them so the SPC mounts; if false, the consumer must install the CSI driver out-of-band (or set secrets.create_secret_provider_class=false)."
  type        = bool
  default     = true
}

variable "labels" {
  description = "Labels applied to the cluster."
  type        = map(string)
  default     = {}
}
