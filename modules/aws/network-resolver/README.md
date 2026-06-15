# `network-resolver` module (AWS)

The single create-vs-lookup branch point for the AWS network path. It emits a **uniform
interface** — `{vpc_id, subnet_ids, pod_subnet_ids, egress_path_ref}` — with identical shape
whether the network was **provisioned** (the `network` module's outputs fed straight through) or
**brought by the customer** (looked up via data sources by VPC ID + subnet tag filters).

- **provision mode** passes the `network` module outputs through unchanged.
- **byo mode** looks up the existing VPC and selects node/pod subnets by tag filter. An empty
  `byo_egress_path_ref` is the deliberate "customer-owned edge" signal that preflight treats as
  amber (the customer owns the perimeter firewall).

Keeping the branch here means every downstream module receives the same inputs regardless of
mode, so no module needs a `create_*` toggle.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.60 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.60 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_subnets.byo_node](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/subnets) | data source |
| [aws_subnets.byo_pod](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/subnets) | data source |
| [aws_vpc.byo](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/vpc) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_byo_egress_path_ref"></a> [byo\_egress\_path\_ref](#input\_byo\_egress\_path\_ref) | Optional customer-supplied egress path reference (byo mode); empty when the customer owns the edge firewall. An empty value is the deliberate "customer-owned edge" signal that preflight treats as amber. | `string` | `""` | no |
| <a name="input_byo_pod_subnet_tag_filter"></a> [byo\_pod\_subnet\_tag\_filter](#input\_byo\_pod\_subnet\_tag\_filter) | Tag key=value used to select the existing pod subnets in byo mode (e.g. { "kubernetes.io/role/cni" = "1" }). Empty selects no separate pod subnets (pods share the node subnets). | `map(string)` | `{}` | no |
| <a name="input_byo_subnet_tag_filter"></a> [byo\_subnet\_tag\_filter](#input\_byo\_subnet\_tag\_filter) | Tag key=value used to select the existing private (node) subnets in byo mode (e.g. { "kubernetes.io/role/internal-elb" = "1" }). | `map(string)` | `{}` | no |
| <a name="input_byo_vpc_id"></a> [byo\_vpc\_id](#input\_byo\_vpc\_id) | Existing VPC ID to look up (byo mode). | `string` | `""` | no |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" feeds the network module's outputs straight through; "byo" looks up an existing VPC/subnets via data sources. | `string` | n/a | yes |
| <a name="input_provisioned_egress_path_ref"></a> [provisioned\_egress\_path\_ref](#input\_provisioned\_egress\_path\_ref) | Egress path ref (Network Firewall ARN) from modules/aws/network (provision mode). | `string` | `""` | no |
| <a name="input_provisioned_pod_subnet_ids"></a> [provisioned\_pod\_subnet\_ids](#input\_provisioned\_pod\_subnet\_ids) | Pod subnet IDs from modules/aws/network (provision mode). Empty when the data plane uses node subnets for pods. | `list(string)` | `[]` | no |
| <a name="input_provisioned_subnet_ids"></a> [provisioned\_subnet\_ids](#input\_provisioned\_subnet\_ids) | Node (private) subnet IDs from modules/aws/network (provision mode). | `list(string)` | `[]` | no |
| <a name="input_provisioned_vpc_id"></a> [provisioned\_vpc\_id](#input\_provisioned\_vpc\_id) | VPC ID from modules/aws/network (provision mode). Ignored in byo mode. | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_egress_path_ref"></a> [egress\_path\_ref](#output\_egress\_path\_ref) | Resolved controlled-egress path reference (Network Firewall ARN, or customer-supplied; empty when the customer owns the edge firewall). |
| <a name="output_pod_subnet_ids"></a> [pod\_subnet\_ids](#output\_pod\_subnet\_ids) | Resolved pod subnet IDs (created or looked up). Empty when pods share the node subnets. |
| <a name="output_subnet_ids"></a> [subnet\_ids](#output\_subnet\_ids) | Resolved node (private) subnet IDs (created or looked up). |
| <a name="output_vpc_id"></a> [vpc\_id](#output\_vpc\_id) | Resolved VPC ID (created or looked up) — identical shape in both modes. |
<!-- END_TF_DOCS -->
