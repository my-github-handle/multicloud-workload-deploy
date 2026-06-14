variable "namespace" {
  description = "Workload namespace to secure."
  type        = string
}

variable "manage_namespace" {
  description = "When true, this module creates+labels the namespace. When false, it assumes the namespace exists (e.g. created by the operator chart in Tier A) and labels it in place."
  type        = bool
  default     = true
}

# There is deliberately NO `control_plane_fqdn` variable. The in-cluster NetworkPolicy floor cannot
# do FQDN-granular egress (plain NetworkPolicy is CIDR/selector-based only), so an FQDN input here
# would be a dead, false-promise input. FQDN scoping is enforced at the perimeter egress firewall
# and, where Cilium is present, via toFQDNs. Here the egress floor is a wide CIDR on the
# control-plane PORT with the metadata IPs carved out. The control-plane PORT is parameterized.
variable "control_plane_port" {
  description = "TCP port for control-plane egress."
  type        = number
  default     = 443
}

variable "workload_port" {
  description = <<-EOT
    The workload's serving port. The namespace-wide allow policy permits intra-namespace ingress to
    this port and intra-namespace egress to it, so the namespace floor does not strangle the
    workload's own traffic. This must match the workload's container port (charts/workload .port /
    WorkloadSpec.port). Set to 0 to omit the workload-port allowances entirely.
  EOT
  type        = number
  default     = 8080
}

variable "dns_namespace" {
  description = "Namespace where cluster DNS (kube-dns/CoreDNS) runs, for the DNS egress allowance."
  type        = string
  default     = "kube-system"
}

variable "psa_enforce_level" {
  description = <<-EOT
    Pod Security Admission enforce level for the workload namespace. Defaults to "restricted" — the
    secure floor for untrusted/financial workloads (non-root, no privilege escalation, dropped
    capabilities). Set to "baseline" only for a workload whose image genuinely must run as root or
    with a writable root filesystem: baseline still blocks privileged containers, host namespaces,
    and hostPath, but permits running as root. "privileged" disables enforcement entirely and
    should not be used for these workloads. audit/warn always track restricted so the gap from the
    secure floor is always visible even when enforce is relaxed.
  EOT
  type        = string
  default     = "restricted"
  validation {
    condition     = contains(["restricted", "baseline", "privileged"], var.psa_enforce_level)
    error_message = "psa_enforce_level must be \"restricted\", \"baseline\", or \"privileged\"."
  }
}

variable "workload_selector_labels" {
  description = <<-EOT
    Pod label selector the network policies apply to. Default is empty ({}), which selects ALL
    pods in the namespace — the canonical namespace-wide default-deny. This is deliberate: the
    workload namespace contains only our pods (workload, connect-agent, and in Tier A the
    namespace-scoped operator), and a namespace-wide deny cannot silently drift from the chart's
    pod-template labels. Do NOT set this to {app.kubernetes.io/managed-by=...}: charts/workload
    applies only `app.kubernetes.io/name=<name>` to the pod template, so a `managed-by` selector
    would match NO pods and render both policies inert. If you must scope, use
    {"app.kubernetes.io/name" = <name>} matching the chart exactly.
  EOT
  type        = map(string)
  default     = {}
}
