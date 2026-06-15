# `kms` module (AWS)

Provides the customer-managed key (CMK) used for envelope encryption across the stack —
Secrets Manager material, EKS secrets at rest, and persistent volumes — folding the
create-vs-lookup decision into this single module.

- **provision mode** creates a CMK with automatic annual rotation (default), a configurable
  deletion window, and an alias.
- **byo mode** resolves a customer-supplied key ARN and fails fast at plan time if the key is not
  enabled.

The resolved `key_arn` is the single reference consumed by the `iam`, `secrets`, and `cluster`
modules, so every consumer encrypts under the same key whether it was created or brought.

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
| [aws_kms_alias.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_alias) | resource |
| [aws_kms_key.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_key) | resource |
| [terraform_data.key_usable](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [aws_kms_key.byo](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/kms_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_alias"></a> [alias](#input\_alias) | KMS alias name (without the alias/ prefix) for the provisioned key. | `string` | `"workload-cmk"` | no |
| <a name="input_deletion_window_days"></a> [deletion\_window\_days](#input\_deletion\_window\_days) | Waiting period before a scheduled key deletion completes (provision mode). | `number` | `30` | no |
| <a name="input_enable_rotation"></a> [enable\_rotation](#input\_enable\_rotation) | Enable automatic annual key rotation (provision mode). | `bool` | `true` | no |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" creates a new CMK with rotation; "byo" resolves a customer-supplied key ARN. | `string` | n/a | yes |
| <a name="input_provided_key_arn"></a> [provided\_key\_arn](#input\_provided\_key\_arn) | Existing CMK ARN to resolve (byo mode). Ignored in provision mode. | `string` | `""` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags applied to the provisioned key. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_alias_name"></a> [alias\_name](#output\_alias\_name) | KMS alias name in provision mode; empty in BYO mode. |
| <a name="output_key_arn"></a> [key\_arn](#output\_key\_arn) | Resolved CMK ARN (created or BYO). |
| <a name="output_key_id"></a> [key\_id](#output\_key\_id) | Resolved CMK key ID. |
<!-- END_TF_DOCS -->
