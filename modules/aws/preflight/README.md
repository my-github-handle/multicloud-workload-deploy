# `preflight` module (AWS) — the two halves

The AWS preflight contract has two co-located parts, by design:

1. **Go `cloud.Provider` (the real staged cloud checks).**
   `operator/internal/cloud/aws` implements the four provider methods —
   `CheckIdentityPermissions` (Stage 0), `CheckKMSKey` (Stage 1),
   `CheckSecretsBackend` (Stage 2), `CheckEgress` (Stage 3) — using `aws-sdk-go-v2`. The
   preflight binary (`operator/cmd/preflight`) selects this provider via `--cloud=aws`, runs the
   staged checks, and emits the green/amber/red report. The shared `modules/preflight` invokes the
   binary and gates `terraform apply` on the verdict. **This is where the permission simulation,
   KMS state, secrets-encryption, and egress-posture checks live.**

2. **This Terraform module (co-located data-source sanity pre-checks).**
   `checks.tf` asserts, inside the plan graph, that the *resolved* resources are coherent: the
   region matches, the VPC is `available`, and the CMK is `enabled`. These are cheap precondition
   guards that fail the plan fast and live next to the modules that produce the inputs. They do
   **not** re-implement the staged report — they backstop it with graph-time assertions.

In `live/aws-full`, the binary runs with `--mode=full --cloud=aws`: stages the greenfield path
satisfies *by provisioning* are informational (downgraded red→amber), not blocking. This module's
preconditions still hard-fail on a genuinely broken resolved resource.

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
| [terraform_data.kms_enabled](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.region_match](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.vpc_available](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_kms_key.resolved](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/kms_key) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |
| [aws_vpc.resolved](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/vpc) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_kms_key_arn"></a> [kms\_key\_arn](#input\_kms\_key\_arn) | Resolved CMK ARN to sanity-check (must be enabled). | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | AWS region (asserted to match the caller's configured region). | `string` | n/a | yes |
| <a name="input_vpc_id"></a> [vpc\_id](#input\_vpc\_id) | Resolved VPC ID to sanity-check (must exist + be available). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_account_id"></a> [account\_id](#output\_account\_id) | The AWS account the deploy is running against (for the report/logs). |
| <a name="output_checks_passed"></a> [checks\_passed](#output\_checks\_passed) | True once all co-located AWS data-source preconditions have evaluated (region/VPC/KMS). |
<!-- END_TF_DOCS -->
