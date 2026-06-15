# `network-resolver` module (GCP)

The single create-vs-lookup branch point for the GCP network path. Emits a uniform
`{vpc_id, subnet_ids, egress_path_ref}` interface — identical shape and types — whether the network
was created or brought, so every downstream module receives the same inputs.

- **provision mode** passes the `network` module's outputs straight through.
- **byo mode** looks up an existing VPC + subnet via data sources; `egress_path_ref` is the
  customer-supplied edge reference (empty when the customer owns the firewall).

On GCP `vpc_id` is the network self-link and `subnet_ids` is a list of subnet self-links.

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

No modules.

## Resources

| Name | Type |
|------|------|
| [google_compute_network.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_network) | data source |
| [google_compute_subnetwork.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_subnetwork) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_byo_egress_path_ref"></a> [byo\_egress\_path\_ref](#input\_byo\_egress\_path\_ref) | Optional customer-supplied egress path reference (byo mode); empty when the customer owns the edge firewall. | `string` | `""` | no |
| <a name="input_byo_network_name"></a> [byo\_network\_name](#input\_byo\_network\_name) | Existing VPC network name to look up (byo mode). | `string` | `""` | no |
| <a name="input_byo_subnet_name"></a> [byo\_subnet\_name](#input\_byo\_subnet\_name) | Existing subnet name to look up (byo mode). | `string` | `""` | no |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" feeds the network module's outputs straight through; "byo" looks up an existing VPC/subnet via data sources. | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID (used by BYO data-source lookups). | `string` | n/a | yes |
| <a name="input_provisioned_egress_path_ref"></a> [provisioned\_egress\_path\_ref](#input\_provisioned\_egress\_path\_ref) | Egress path ref (firewall policy name) from modules/gcp/network (provision mode). | `string` | `""` | no |
| <a name="input_provisioned_network_self_link"></a> [provisioned\_network\_self\_link](#input\_provisioned\_network\_self\_link) | Network self-link from modules/gcp/network (provision mode). Ignored in byo mode. | `string` | `""` | no |
| <a name="input_provisioned_subnet_self_links"></a> [provisioned\_subnet\_self\_links](#input\_provisioned\_subnet\_self\_links) | Subnet self-links from modules/gcp/network (provision mode). | `list(string)` | `[]` | no |
| <a name="input_region"></a> [region](#input\_region) | Region of the subnet (BYO lookup). | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_egress_path_ref"></a> [egress\_path\_ref](#output\_egress\_path\_ref) | Resolved controlled-egress path reference (firewall policy name, or customer-supplied). |
| <a name="output_subnet_ids"></a> [subnet\_ids](#output\_subnet\_ids) | Resolved subnet self-links (created or looked up). |
| <a name="output_vpc_id"></a> [vpc\_id](#output\_vpc\_id) | Resolved VPC network self-link (created or looked up) — identical shape in both modes. (GCP networks are referenced by self\_link, the cross-module identifier.) |
<!-- END_TF_DOCS -->
