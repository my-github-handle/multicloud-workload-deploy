# `network` module (GCP)

Provisions the GCP network foundation for a hardened, private GKE cluster: a VPC with secondary
alias ranges, controlled egress, private DNS for Google APIs, and an immutable audit floor.

- **Data plane on the CGNAT block.** A single subnet whose primary range carries nodes, with two
  secondary alias ranges for GKE pods and services — all drawn from `100.64.0.0/16` so pod/service
  IP churn never exhausts routable space.
- **Controlled egress, default-deny.** A VPC network firewall policy drops all egress except an
  explicit allowlist: the GKE control-plane CIDR, intra-VPC ranges, the restricted Google APIs VIP,
  DNS via the metadata server, and the configured FQDNs/CIDRs. Egress leaves through a Cloud NAT on
  a Cloud Router (no public node IPs).
- **Private DNS for restricted Private Google Access.** Private managed zones map
  `googleapis.com` / `gcr.io` / `pkg.dev` to the restricted VIP, so nodes resolve Google APIs to
  the allowlisted VIP instead of public IPs that default-deny would drop.
- **Always-on audit floor.** Subnet VPC Flow Logs route through a Logging sink to a customer-owned
  Cloud Storage bucket with a locked retention policy — immutable, CNI-independent.

The create-vs-lookup decision lives in the companion `network-resolver`; this module always
provisions.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_google"></a> [google](#requirement\_google) | ~> 6.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | ~> 6.0 |

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_vpc"></a> [vpc](#module\_vpc) | terraform-google-modules/network/google | ~> 9.0 |

## Resources

| Name | Type |
|------|------|
| [google_compute_network_firewall_policy.egress](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy) | resource |
| [google_compute_network_firewall_policy_association.egress](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_association) | resource |
| [google_compute_network_firewall_policy_rule.egress_cidr_allow](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_control_plane](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_default_deny](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_dns](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_fqdn_allow](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_google_apis](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_network_firewall_policy_rule.egress_intra_vpc](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network_firewall_policy_rule) | resource |
| [google_compute_router.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_router) | resource |
| [google_compute_router_nat.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_router_nat) | resource |
| [google_dns_managed_zone.google_apis](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/dns_managed_zone) | resource |
| [google_dns_record_set.apex_a](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/dns_record_set) | resource |
| [google_dns_record_set.wildcard_cname](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/dns_record_set) | resource |
| [google_logging_project_sink.flow_logs](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/logging_project_sink) | resource |
| [google_storage_bucket.flow_logs](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/storage_bucket) | resource |
| [google_storage_bucket_iam_member.flow_logs_writer](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/storage_bucket_iam_member) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_egress_allowed_cidrs"></a> [egress\_allowed\_cidrs](#input\_egress\_allowed\_cidrs) | CIDR blocks allowed for egress (e.g. Google API ranges not covered by FQDN rules). | `list(string)` | `[]` | no |
| <a name="input_egress_allowed_fqdns"></a> [egress\_allowed\_fqdns](#input\_egress\_allowed\_fqdns) | FQDNs allowed for egress through the VPC firewall policy (control-plane FQDN, ghcr.io, Google API endpoints, observability sinks). Everything else is default-deny. | `list(string)` | <pre>[<br/>  "ghcr.io",<br/>  "github.com"<br/>]</pre> | no |
| <a name="input_flow_log_retention_days"></a> [flow\_log\_retention\_days](#input\_flow\_log\_retention\_days) | Bucket-lock retention (days) on the customer-owned VPC Flow Logs GCS bucket. The always-on audit floor. | `number` | `365` | no |
| <a name="input_google_api_cidrs"></a> [google\_api\_cidrs](#input\_google\_api\_cidrs) | Private Google Access VIP range(s). Under default-deny egress these MUST be allowed or nodes cannot reach Artifact Registry/KMS/Secret Manager/Workload-Identity and will not register. Default is the RESTRICTED VIP 199.36.153.4/30 (use restricted.googleapis.com). The speculative anycast 34.126.0.0/18 is deliberately DROPPED; add the private VIP 199.36.153.8/30 only if not using the restricted VIP for all Google APIs. | `list(string)` | <pre>[<br/>  "199.36.153.4/30"<br/>]</pre> | no |
| <a name="input_intra_vpc_cidrs"></a> [intra\_vpc\_cidrs](#input\_intra\_vpc\_cidrs) | Intra-VPC ranges (subnet + pod + service CIDRs) that pod-to-pod / pod-to-node / pod-to-service traffic needs under default-deny egress. Defaults to the three CIDRs this module manages; override if BYO ranges differ. | `list(string)` | <pre>[<br/>  "100.64.0.0/18",<br/>  "100.64.128.0/17",<br/>  "100.64.64.0/19"<br/>]</pre> | no |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels applied to all resources that support them. | `map(string)` | `{}` | no |
| <a name="input_master_ipv4_cidr_block"></a> [master\_ipv4\_cidr\_block](#input\_master\_ipv4\_cidr\_block) | The /28 CIDR of the GKE private control-plane endpoint. Under default-deny egress this MUST be allowed or nodes cannot reach the control plane and the cluster bricks. Must match the value passed to the cluster module. | `string` | `"172.16.0.0/28"` | no |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for all network resources. | `string` | n/a | yes |
| <a name="input_pods_cidr"></a> [pods\_cidr](#input\_pods\_cidr) | Secondary range CIDR for GKE pods (alias IPs). The largest data-plane sub-block. | `string` | `"100.64.128.0/17"` | no |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID the network lives in. | `string` | n/a | yes |
| <a name="input_project_number"></a> [project\_number](#input\_project\_number) | GCP project number (from the project module). Used to name the flow-log bucket without re-reading the project, so the module does not depend on the project existing at plan time. | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | GCP region for the subnet, router, and NAT. | `string` | `"us-central1"` | no |
| <a name="input_services_cidr"></a> [services\_cidr](#input\_services\_cidr) | Secondary range CIDR for GKE services (alias IPs). A small data-plane sub-block. | `string` | `"100.64.64.0/19"` | no |
| <a name="input_subnet_cidr"></a> [subnet\_cidr](#input\_subnet\_cidr) | Primary CIDR of the node subnet (no public IPs on nodes). | `string` | `"100.64.0.0/18"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_egress_path_ref"></a> [egress\_path\_ref](#output\_egress\_path\_ref) | Reference to the controlled egress path (the network firewall policy name). The resolver re-exports this uniformly. |
| <a name="output_flow_log_bucket"></a> [flow\_log\_bucket](#output\_flow\_log\_bucket) | Name of the customer-owned, retention-locked GCS bucket holding VPC Flow Logs (the always-on audit floor). |
| <a name="output_network_id"></a> [network\_id](#output\_network\_id) | ID of the provisioned VPC network. |
| <a name="output_network_self_link"></a> [network\_self\_link](#output\_network\_self\_link) | Self-link of the provisioned VPC network (the GCP analogue of vpc\_id). |
| <a name="output_pods_range_name"></a> [pods\_range\_name](#output\_pods\_range\_name) | Secondary range name for GKE pods (alias IPs). |
| <a name="output_router_name"></a> [router\_name](#output\_router\_name) | Cloud Router name whose Cloud NAT provides the controlled egress path (consumed by the preflight egress check). |
| <a name="output_services_range_name"></a> [services\_range\_name](#output\_services\_range\_name) | Secondary range name for GKE services (alias IPs). |
| <a name="output_subnet_id"></a> [subnet\_id](#output\_subnet\_id) | ID of the node subnet. |
| <a name="output_subnet_self_link"></a> [subnet\_self\_link](#output\_subnet\_self\_link) | Self-link of the node subnet (nodes have no public IPs). |
<!-- END_TF_DOCS -->
