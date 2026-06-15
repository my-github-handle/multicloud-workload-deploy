# `network` module (AWS)

Provisions the AWS network foundation for a hardened, private cluster: a VPC with a
**primary/secondary CIDR split**, per-AZ HA, an in-path egress firewall, and an immutable audit
floor.

- **Primary CIDR — edge only.** Public subnets (NAT gateways, load balancers) and dedicated
  Network Firewall endpoint subnets. Small and stable; it never grows with the workload.
- **Secondary CIDR — data plane.** Node subnets and pod subnets in a large CGNAT-range block, so
  node/pod IP churn never exhausts the routable primary CIDR. The pod subnets are tagged
  (`kubernetes.io/role/cni` by default) for Cilium ENI-mode pod-IP allocation.
- **High availability.** One NAT gateway, one Network Firewall endpoint, and one data-plane route
  table **per availability zone**. At least two AZs are required; the loss of one AZ never strands
  another's egress.
- **Egress firewall, default-deny.** AWS Network Firewall with a STRICT_ORDER stateful policy
  (`aws:drop_established` default). The FQDN allowlist is rendered as STRICT_ORDER-compatible
  Suricata `pass` rules (TLS-SNI + HTTP-Host per allowed FQDN); an optional CIDR allow group
  covers service prefixes. Each node/pod subnet's default route points at its own AZ's firewall
  endpoint, so the firewall is genuinely in path.
- **Always-on audit floor.** VPC Flow Logs capture all traffic to a customer-owned S3 bucket with
  COMPLIANCE-mode Object Lock — immutable for the retention period, CNI-independent, and able to
  survive cluster compromise.

The single create-vs-lookup decision lives in the companion `network-resolver` module, not here;
this module always provisions.

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
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_eip.nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eip) | resource |
| [aws_flow_log.vpc](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/flow_log) | resource |
| [aws_internet_gateway.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/internet_gateway) | resource |
| [aws_nat_gateway.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/nat_gateway) | resource |
| [aws_networkfirewall_firewall.egress](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/networkfirewall_firewall) | resource |
| [aws_networkfirewall_firewall_policy.egress](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/networkfirewall_firewall_policy) | resource |
| [aws_networkfirewall_rule_group.egress_allowlist](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/networkfirewall_rule_group) | resource |
| [aws_networkfirewall_rule_group.egress_cidr_allow](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/networkfirewall_rule_group) | resource |
| [aws_route.data_egress_via_firewall](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route.firewall_to_nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route.public_internet](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route_table.data](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table.firewall](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table_association.firewall](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_route_table_association.node](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_route_table_association.pod](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_route_table_association.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_s3_bucket.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket) | resource |
| [aws_s3_bucket_object_lock_configuration.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket_object_lock_configuration) | resource |
| [aws_s3_bucket_policy.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket_policy) | resource |
| [aws_s3_bucket_public_access_block.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket_public_access_block) | resource |
| [aws_s3_bucket_server_side_encryption_configuration.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket_server_side_encryption_configuration) | resource |
| [aws_s3_bucket_versioning.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket_versioning) | resource |
| [aws_subnet.firewall](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.node](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.pod](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_vpc.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc) | resource |
| [aws_vpc_ipv4_cidr_block_association.secondary](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_ipv4_cidr_block_association) | resource |
| [terraform_data.cidr_parity](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_iam_policy_document.flow_logs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_azs"></a> [azs](#input\_azs) | Availability zones to spread subnets across. At least two for HA — every NAT gateway, firewall endpoint, and data-plane route table is provisioned per-AZ so the loss of one AZ never strands another's egress. | `list(string)` | n/a | yes |
| <a name="input_egress_allowed_cidrs"></a> [egress\_allowed\_cidrs](#input\_egress\_allowed\_cidrs) | CIDR blocks allowed for egress (e.g. AWS service prefixes not covered by FQDN rules). | `list(string)` | `[]` | no |
| <a name="input_egress_allowed_fqdns"></a> [egress\_allowed\_fqdns](#input\_egress\_allowed\_fqdns) | FQDNs allowed for egress through the Network Firewall (control-plane FQDN, ghcr.io, AWS API endpoints, observability sinks). Everything else is default-deny. | `list(string)` | <pre>[<br/>  "ghcr.io",<br/>  "github.com"<br/>]</pre> | no |
| <a name="input_firewall_subnet_cidrs"></a> [firewall\_subnet\_cidrs](#input\_firewall\_subnet\_cidrs) | Dedicated AWS Network Firewall endpoint subnet CIDRs (primary CIDR), one per AZ. AWS requires the firewall endpoint to live in its own subnet — placing it in a node subnet creates a routing loop. Small /28s suffice (one ENI per AZ). | `list(string)` | <pre>[<br/>  "10.0.0.128/28",<br/>  "10.0.0.144/28",<br/>  "10.0.0.160/28"<br/>]</pre> | no |
| <a name="input_flow_log_retention_days"></a> [flow\_log\_retention\_days](#input\_flow\_log\_retention\_days) | Object-lock retention (days) on the customer-owned VPC Flow Logs S3 bucket. The always-on audit floor. | `number` | `365` | no |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for all network resources. | `string` | n/a | yes |
| <a name="input_node_subnet_cidrs"></a> [node\_subnet\_cidrs](#input\_node\_subnet\_cidrs) | Node subnet CIDRs (secondary CIDR), one per AZ — private, no public IPs. Default route is forced through the same-AZ Network Firewall endpoint. | `list(string)` | <pre>[<br/>  "100.64.0.0/18",<br/>  "100.64.64.0/18",<br/>  "100.64.128.0/18"<br/>]</pre> | no |
| <a name="input_pod_subnet_cidrs"></a> [pod\_subnet\_cidrs](#input\_pod\_subnet\_cidrs) | Pod subnet CIDRs (secondary CIDR), one per AZ. The VPC CNI in custom-networking mode allocates pod IPs from these subnets (discovered via the kubernetes.io/role/cni tag), isolating pod IP churn from the node subnets. Egress is forced through the same-AZ firewall endpoint, identical to the node subnets. | `list(string)` | <pre>[<br/>  "100.64.192.0/19",<br/>  "100.64.224.0/19",<br/>  "100.65.0.0/19"<br/>]</pre> | no |
| <a name="input_pod_subnet_tags"></a> [pod\_subnet\_tags](#input\_pod\_subnet\_tags) | Tags applied to the pod subnets so the CNI discovers them. The default tags the subnets for Cilium ENI-mode allocation (kubernetes.io/role/cni). | `map(string)` | <pre>{<br/>  "kubernetes.io/role/cni": "1"<br/>}</pre> | no |
| <a name="input_public_subnet_cidrs"></a> [public\_subnet\_cidrs](#input\_public\_subnet\_cidrs) | Public subnet CIDRs (primary CIDR), one per AZ — NAT gateways and load balancers. | `list(string)` | <pre>[<br/>  "10.0.0.0/27",<br/>  "10.0.0.32/27",<br/>  "10.0.0.64/27"<br/>]</pre> | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags applied to all resources. | `map(string)` | `{}` | no |
| <a name="input_vpc_primary_cidr"></a> [vpc\_primary\_cidr](#input\_vpc\_primary\_cidr) | Primary VPC CIDR. Carries only the edge tiers — public subnets (NAT gateways, load balancers) and the dedicated Network Firewall endpoint subnets. Kept small and stable; it never grows with the workload. | `string` | `"10.0.0.0/24"` | no |
| <a name="input_vpc_secondary_cidr"></a> [vpc\_secondary\_cidr](#input\_vpc\_secondary\_cidr) | Secondary VPC CIDR for the data plane (node and pod subnets). A large CGNAT-range block so nodes and Cilium-managed pods never exhaust the routable primary CIDR. Associated as a secondary IPv4 block on the VPC. | `string` | `"100.64.0.0/16"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_egress_path_ref"></a> [egress\_path\_ref](#output\_egress\_path\_ref) | Reference to the controlled egress path (the Network Firewall ARN). The resolver re-exports this uniformly. |
| <a name="output_firewall_subnet_ids"></a> [firewall\_subnet\_ids](#output\_firewall\_subnet\_ids) | Dedicated Network Firewall endpoint subnet IDs (primary CIDR, auto-routed to NAT). |
| <a name="output_flow_log_bucket_arn"></a> [flow\_log\_bucket\_arn](#output\_flow\_log\_bucket\_arn) | ARN of the customer-owned, retention-locked S3 bucket holding VPC Flow Logs (the always-on audit floor). |
| <a name="output_pod_subnet_ids"></a> [pod\_subnet\_ids](#output\_pod\_subnet\_ids) | Pod subnet IDs (secondary CIDR) from which Cilium ENI mode allocates pod IPs. |
| <a name="output_private_subnet_ids"></a> [private\_subnet\_ids](#output\_private\_subnet\_ids) | Node subnet IDs (secondary CIDR, egress forced through the Network Firewall). Where the cluster module places nodes; named private\_subnet\_ids for cross-cloud resolver uniformity. |
| <a name="output_public_subnet_ids"></a> [public\_subnet\_ids](#output\_public\_subnet\_ids) | Public (NAT/LB) subnet IDs (primary CIDR). |
| <a name="output_vpc_id"></a> [vpc\_id](#output\_vpc\_id) | Provisioned VPC ID. |
<!-- END_TF_DOCS -->
