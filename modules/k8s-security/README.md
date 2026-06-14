# `k8s-security` module

Applies the in-cluster security floor to the workload namespace.

- **PodSecurity admission**: labels the namespace with a configurable enforce level
  (`psa_enforce_level`, default `restricted`; `baseline` permits images that must run as root).
  `audit`/`warn` always track `restricted` so the gap from the secure floor stays visible. Creates
  the namespace with the labels (`manage_namespace = true`) or labels an existing one in place.
- **Default-deny NetworkPolicy**: a namespace-wide policy denying all ingress and egress.
- **Allow NetworkPolicy**: re-opens only what the workload needs — DNS, the control-plane port on a
  wide CIDR with the cloud metadata IPs carved out (a credential-theft block), and intra-namespace
  ingress/egress on the workload port so the namespace floor does not strangle the workload's own
  serving traffic.

FQDN-granular egress is intentionally not attempted here (plain NetworkPolicy is CIDR/port-based);
that is the perimeter firewall's / Cilium `toFQDNs`' job.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.30 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_kubernetes"></a> [kubernetes](#provider\_kubernetes) | ~> 2.30 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [kubernetes_labels.psa](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/labels) | resource |
| [kubernetes_namespace.this](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/namespace) | resource |
| [kubernetes_network_policy.allow](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/network_policy) | resource |
| [kubernetes_network_policy.default_deny](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/network_policy) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_control_plane_port"></a> [control\_plane\_port](#input\_control\_plane\_port) | TCP port for control-plane egress. | `number` | `443` | no |
| <a name="input_dns_namespace"></a> [dns\_namespace](#input\_dns\_namespace) | Namespace where cluster DNS (kube-dns/CoreDNS) runs, for the DNS egress allowance. | `string` | `"kube-system"` | no |
| <a name="input_manage_namespace"></a> [manage\_namespace](#input\_manage\_namespace) | When true, this module creates+labels the namespace. When false, it assumes the namespace exists (e.g. created by the operator chart in Tier A) and labels it in place. | `bool` | `true` | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Workload namespace to secure. | `string` | n/a | yes |
| <a name="input_psa_enforce_level"></a> [psa\_enforce\_level](#input\_psa\_enforce\_level) | Pod Security Admission enforce level for the workload namespace. Defaults to "restricted" — the<br/>secure floor for untrusted/financial workloads (non-root, no privilege escalation, dropped<br/>capabilities). Set to "baseline" only for a workload whose image genuinely must run as root or<br/>with a writable root filesystem: baseline still blocks privileged containers, host namespaces,<br/>and hostPath, but permits running as root. "privileged" disables enforcement entirely and<br/>should not be used for these workloads. audit/warn always track restricted so the gap from the<br/>secure floor is always visible even when enforce is relaxed. | `string` | `"restricted"` | no |
| <a name="input_workload_port"></a> [workload\_port](#input\_workload\_port) | The workload's serving port. The namespace-wide allow policy permits intra-namespace ingress to<br/>this port and intra-namespace egress to it, so the namespace floor does not strangle the<br/>workload's own traffic. This must match the workload's container port (charts/workload .port /<br/>WorkloadSpec.port). Set to 0 to omit the workload-port allowances entirely. | `number` | `8080` | no |
| <a name="input_workload_selector_labels"></a> [workload\_selector\_labels](#input\_workload\_selector\_labels) | Pod label selector the network policies apply to. Default is empty ({}), which selects ALL<br/>pods in the namespace — the canonical namespace-wide default-deny. This is deliberate: the<br/>workload namespace contains only our pods (workload, connect-agent, and in Tier A the<br/>namespace-scoped operator), and a namespace-wide deny cannot silently drift from the chart's<br/>pod-template labels. Do NOT set this to {app.kubernetes.io/managed-by=...}: charts/workload<br/>applies only `app.kubernetes.io/name=<name>` to the pod template, so a `managed-by` selector<br/>would match NO pods and render both policies inert. If you must scope, use<br/>{"app.kubernetes.io/name" = <name>} matching the chart exactly. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_namespace"></a> [namespace](#output\_namespace) | The secured namespace. |
| <a name="output_network_policy_names"></a> [network\_policy\_names](#output\_network\_policy\_names) | Names of the NetworkPolicies applied to the namespace. |
<!-- END_TF_DOCS -->
