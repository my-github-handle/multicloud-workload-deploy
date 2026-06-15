# `iam` module (AWS)

Renders the workload's identity and its least-privilege policies, and either creates the IRSA
role or resolves a customer-supplied one.

- **Runtime workload identity.** An IRSA role trusted only by the workload ServiceAccount in the
  workload namespace (via the cluster's OIDC provider). Its policy grants the minimum the workload
  needs: `kms:Decrypt`/`GenerateDataKey`/`DescribeKey` on the resolved CMK ARN only,
  `secretsmanager:GetSecretValue`/`DescribeSecret` on the secret path prefix only, and ECR pull on
  the named repos. No service wildcards; region pinned.
- **Deploy-time identity.** The create/manage permission set for the greenfield path (KMS,
  Secrets Manager, IAM role, EKS), scoped by account/region and resource ARN prefix. Rendered so a
  customer can review what the `terraform apply` operator needs.
- **Reviewable artifacts.** Both policy documents plus the trust policy are written as JSON files
  and exposed as outputs, so they can be inspected before anything is granted.
- **byo-identity mode.** If the customer creates the role themselves, the module emits the exact
  policy + trust documents for them to attach and resolves the supplied role ARN — no role is
  created.

Secrets Manager access is scoped at the **path prefix** (`<prefix>-*`) rather than per-secret ARN.
That keeps this module independent of the `secrets` module (no dependency cycle) while still
covering every secret created under the prefix.

> The rendered `artifacts/*.json` files are produced at apply time and are gitignored; the
> `*.tf` here is the source of truth, and `tests/no_wildcards.tftest.hcl` asserts both policies
> stay wildcard-free and resource-scoped.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.60 |
| <a name="requirement_local"></a> [local](#requirement\_local) | ~> 2.5 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.60 |
| <a name="provider_local"></a> [local](#provider\_local) | ~> 2.5 |
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_iam_role.workload](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.runtime](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [local_file.deploy_policy](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.recorded_secret_arns](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.runtime_policy](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.trust_policy](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [terraform_data.mode_inputs](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [aws_iam_policy_document.deploy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.runtime](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.trust](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_role.byo](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_role) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_account_id"></a> [account\_id](#input\_account\_id) | AWS account ID — pinned into policy conditions. | `string` | n/a | yes |
| <a name="input_artifacts_dir"></a> [artifacts\_dir](#input\_artifacts\_dir) | Directory (relative to the module) to write the reviewable policy JSON artifacts into. | `string` | `"artifacts"` | no |
| <a name="input_ecr_repo_arns"></a> [ecr\_repo\_arns](#input\_ecr\_repo\_arns) | ECR repository ARNs the workload pulls from. The runtime policy scopes ECR pull actions to these repos only. | `list(string)` | `[]` | no |
| <a name="input_kms_key_arn"></a> [kms\_key\_arn](#input\_kms\_key\_arn) | Resolved CMK ARN (from the kms module). The runtime policy scopes kms:Decrypt/GenerateDataKey to THIS ARN only — no wildcards. | `string` | n/a | yes |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" creates the IRSA role and attaches the rendered policies; "byo" emits the policy+trust docs and resolves a customer-supplied role ARN. | `string` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for the IAM role and policies. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace of the workload + connect-agent ServiceAccount. | `string` | n/a | yes |
| <a name="input_oidc_issuer_url"></a> [oidc\_issuer\_url](#input\_oidc\_issuer\_url) | EKS OIDC issuer URL (https://oidc.eks...). The scheme is stripped internally to form the sub/aud condition keys. | `string` | n/a | yes |
| <a name="input_oidc_provider_arn"></a> [oidc\_provider\_arn](#input\_oidc\_provider\_arn) | ARN of the EKS cluster's IAM OIDC provider (from the cluster module), for the IRSA trust policy. | `string` | n/a | yes |
| <a name="input_provided_role_arn"></a> [provided\_role\_arn](#input\_provided\_role\_arn) | Existing IRSA role ARN to resolve (byo-identity mode). The module still emits the policy+trust docs for the customer to attach. | `string` | `""` | no |
| <a name="input_recorded_secret_arns"></a> [recorded\_secret\_arns](#input\_recorded\_secret\_arns) | Optional concrete secret ARNs recorded in a companion artifact for reviewer visibility only. The live policy is scoped by secret\_path\_prefix, not these ARNs; may be left empty. | `list(string)` | `[]` | no |
| <a name="input_region"></a> [region](#input\_region) | AWS region — pinned into policy conditions (no account-wide grants). | `string` | n/a | yes |
| <a name="input_secret_path_prefix"></a> [secret\_path\_prefix](#input\_secret\_path\_prefix) | Secrets Manager name prefix the workload's secrets live under (matches the name prefix passed to modules/aws/secrets). The runtime policy scopes secretsmanager:GetSecretValue to arn:...:secret:<prefix>-* — path-prefix scoped, not per-secret-arn. | `string` | n/a | yes |
| <a name="input_service_account"></a> [service\_account](#input\_service\_account) | Kubernetes ServiceAccount name bound to this role (workload + connect-agent). | `string` | `"workload"` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags applied to created IAM resources. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_deploy_policy_json"></a> [deploy\_policy\_json](#output\_deploy\_policy\_json) | Rendered deploy-time identity policy (reviewable artifact). |
| <a name="output_role_arn"></a> [role\_arn](#output\_role\_arn) | Resolved IRSA role ARN (created in provision mode, looked up in byo-identity mode). |
| <a name="output_runtime_policy_json"></a> [runtime\_policy\_json](#output\_runtime\_policy\_json) | Rendered runtime workload identity policy — scoped to the resolved KMS key ARN and the Secrets Manager path prefix (<prefix>-*, not per-arn), no wildcards except the non-scopable ecr:GetAuthorizationToken. |
| <a name="output_trust_policy_json"></a> [trust\_policy\_json](#output\_trust\_policy\_json) | Rendered IRSA trust/assume-role policy (reviewable artifact; the doc to attach in byo-identity mode). |
| <a name="output_workload_identity_ref"></a> [workload\_identity\_ref](#output\_workload\_identity\_ref) | Workload identity reference — the IRSA role ARN annotation value for the ServiceAccount (eks.amazonaws.com/role-arn). |
<!-- END_TF_DOCS -->
