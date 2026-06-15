# `cluster` module (AWS)

Provisions a hardened, private EKS cluster with the IRSA OIDC issuer, CMK secrets envelope
encryption, and full control-plane logging — hand-rolled from the underlying resources for full
control over the data plane.

- **Private by default.** The API endpoint has private access enabled and public access disabled;
  `endpoint_public_access` opts back in (with `public_access_cidrs`) only when explicitly set.
- **CMK secrets encryption.** Cluster secrets are envelope-encrypted at rest with the resolved CMK
  from the `kms` module. The cluster role is granted use of that key.
- **IRSA.** An IAM OIDC provider is created from the cluster's OIDC issuer, so the `iam` module's
  trust policy can federate the workload ServiceAccount.
- **Virtual Service CIDR.** `service_ipv4_cidr` (default `172.20.0.0/16`) must not overlap either
  VPC CIDR — ClusterIP Service addresses are virtual, never real VPC IPs.
- **Private node group.** A managed node group runs only in the resolved node subnets (no public
  IPs). The node role carries the standard worker + CNI + ECR-read policies.
- **VPC CNI custom networking.** The `vpc-cni` addon is installed **before** the node group with
  custom-networking `configuration_values`: it sets the `aws-node` env and creates one `ENIConfig`
  per AZ so pods draw secondary ENIs from the **pod subnets** (secondary CIDR) while the node
  primary ENI stays in the node subnet. `ENABLE_PREFIX_DELEGATION` recovers the max-pods lost when
  the primary ENI no longer serves pods. `coredns` + `kube-proxy` addons are installed too.

Because the VPC CNI is the default EKS CNI and is present from the start, **nodes reach `Ready`
immediately** — there is no bootstrap gap. Pass `pod_subnet_ids` + `node_azs` (positionally
aligned) to enable custom networking; leave `pod_subnet_ids` empty to let pods share the node
subnets. Cilium, when enabled at the root, runs in **chaining mode on top of** the VPC CNI for
NetworkPolicy/Hubble/toFQDNs — it does not own IPAM, so it never gates node readiness.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.60 |
| <a name="requirement_tls"></a> [tls](#requirement\_tls) | ~> 4.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.60 |
| <a name="provider_tls"></a> [tls](#provider\_tls) | ~> 4.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_eks_addon.coredns](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_addon) | resource |
| [aws_eks_addon.kube_proxy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_addon) | resource |
| [aws_eks_addon.vpc_cni](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_addon) | resource |
| [aws_eks_cluster.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_cluster) | resource |
| [aws_eks_node_group.default](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_node_group) | resource |
| [aws_iam_openid_connect_provider.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_openid_connect_provider) | resource |
| [aws_iam_role.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.node](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.cluster_kms](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy_attachment.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_iam_role_policy_attachment.node](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_iam_policy_document.cluster_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.cluster_kms](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.node_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |
| [tls_certificate.oidc](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/certificate) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_coredns_addon_version"></a> [coredns\_addon\_version](#input\_coredns\_addon\_version) | Pinned coredns EKS addon version (empty = default). | `string` | `""` | no |
| <a name="input_enabled_cluster_log_types"></a> [enabled\_cluster\_log\_types](#input\_enabled\_cluster\_log\_types) | Control-plane log types streamed to CloudWatch (audit-grade). | `list(string)` | <pre>[<br/>  "api",<br/>  "audit",<br/>  "authenticator",<br/>  "controllerManager",<br/>  "scheduler"<br/>]</pre> | no |
| <a name="input_endpoint_public_access"></a> [endpoint\_public\_access](#input\_endpoint\_public\_access) | Expose the EKS API endpoint publicly. Default false — a private cluster reachable only from within the VPC. | `bool` | `false` | no |
| <a name="input_k8s_version"></a> [k8s\_version](#input\_k8s\_version) | Kubernetes version for the cluster. | `string` | `"1.34"` | no |
| <a name="input_kms_key_arn"></a> [kms\_key\_arn](#input\_kms\_key\_arn) | Resolved CMK ARN (from the kms module) for EKS secrets envelope encryption at rest. | `string` | n/a | yes |
| <a name="input_kube_proxy_addon_version"></a> [kube\_proxy\_addon\_version](#input\_kube\_proxy\_addon\_version) | Pinned kube-proxy EKS addon version (empty = default). | `string` | `""` | no |
| <a name="input_name"></a> [name](#input\_name) | EKS cluster name. | `string` | n/a | yes |
| <a name="input_node_azs"></a> [node\_azs](#input\_node\_azs) | Availability zones aligned positionally with pod\_subnet\_ids (index i of pod\_subnet\_ids is in node\_azs[i]). Used to build the per-AZ ENIConfig map keyed by the topology.kubernetes.io/zone label. Required when pod\_subnet\_ids is set. | `list(string)` | `[]` | no |
| <a name="input_node_desired_size"></a> [node\_desired\_size](#input\_node\_desired\_size) | Desired managed node group size. | `number` | `2` | no |
| <a name="input_node_instance_types"></a> [node\_instance\_types](#input\_node\_instance\_types) | Managed node group instance types. | `list(string)` | <pre>[<br/>  "m6i.large"<br/>]</pre> | no |
| <a name="input_node_max_size"></a> [node\_max\_size](#input\_node\_max\_size) | Maximum managed node group size. | `number` | `5` | no |
| <a name="input_node_min_size"></a> [node\_min\_size](#input\_node\_min\_size) | Minimum managed node group size. | `number` | `2` | no |
| <a name="input_pod_subnet_ids"></a> [pod\_subnet\_ids](#input\_pod\_subnet\_ids) | Resolved pod subnet IDs (from network-resolver), one per AZ in the secondary CIDR. VPC CNI custom networking places pod secondary ENIs here via a per-AZ ENIConfig, keeping pods off the node subnet's address space. Empty disables custom networking (pods share the node subnets). | `list(string)` | `[]` | no |
| <a name="input_private_subnet_ids"></a> [private\_subnet\_ids](#input\_private\_subnet\_ids) | Resolved node subnet IDs (from network-resolver) — nodes have no public IPs. Spread across AZs for HA. | `list(string)` | n/a | yes |
| <a name="input_public_access_cidrs"></a> [public\_access\_cidrs](#input\_public\_access\_cidrs) | CIDRs allowed to reach the public API endpoint when endpoint\_public\_access is true. Ignored when the endpoint is private. | `list(string)` | `[]` | no |
| <a name="input_service_ipv4_cidr"></a> [service\_ipv4\_cidr](#input\_service\_ipv4\_cidr) | Virtual ClusterIP Service CIDR. Must NOT overlap either VPC CIDR — Service IPs are never real VPC IPs. | `string` | `"172.20.0.0/16"` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags applied to the cluster resources. | `map(string)` | `{}` | no |
| <a name="input_vpc_cni_addon_version"></a> [vpc\_cni\_addon\_version](#input\_vpc\_cni\_addon\_version) | Pinned vpc-cni EKS addon version (empty = the cluster's default version). | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_ca"></a> [ca](#output\_ca) | Base64-encoded cluster CA certificate. |
| <a name="output_cluster_name"></a> [cluster\_name](#output\_cluster\_name) | EKS cluster name. |
| <a name="output_cluster_security_group_id"></a> [cluster\_security\_group\_id](#output\_cluster\_security\_group\_id) | The EKS-managed cluster security group ID. |
| <a name="output_endpoint"></a> [endpoint](#output\_endpoint) | EKS API server endpoint (private). |
| <a name="output_node_role_arn"></a> [node\_role\_arn](#output\_node\_role\_arn) | Managed node group IAM role ARN. |
| <a name="output_oidc_issuer_url"></a> [oidc\_issuer\_url](#output\_oidc\_issuer\_url) | OIDC issuer URL — consumed by the iam module for the sub/aud conditions. |
| <a name="output_oidc_provider_arn"></a> [oidc\_provider\_arn](#output\_oidc\_provider\_arn) | IAM OIDC provider ARN — consumed by the iam module's IRSA trust policy. |
<!-- END_TF_DOCS -->
