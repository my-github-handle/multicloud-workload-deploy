# `secrets` module (AWS)

Creates the workload's secrets in AWS Secrets Manager, envelope-encrypted with the resolved CMK,
and renders the Secrets Store CSI `SecretProviderClass` the workload pod mounts.

- **CMK envelope encryption.** Each secret is encrypted with the `kms_key_arn` from the `kms`
  module, so secret material at rest is protected by the customer-managed key.
- **Secrets Store CSI wiring.** A `SecretProviderClass` referencing the AWS provider is applied as
  raw YAML (`kubectl_manifest`), so the module plans offline and applies once the CSI driver's CRD
  is present — no plan-time CRD discovery. The CSI driver + AWS provider DaemonSet are installed at
  cluster bootstrap; this module renders only the per-workload class.
- **Greenfield two-phase toggle.** `create_secret_provider_class = false` creates the secrets
  without applying the `SecretProviderClass` — used in greenfield Phase 1, before the CSI CRD
  exists on the freshly provisioned cluster.

The workload's IRSA role (from the `iam` module) grants `GetSecretValue` at the `<name>-*` path
prefix, covering every secret this module creates without a module dependency between `iam` and
`secrets`.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 5.60 |
| <a name="requirement_kubectl"></a> [kubectl](#requirement\_kubectl) | ~> 2.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 5.60 |
| <a name="provider_kubectl"></a> [kubectl](#provider\_kubectl) | ~> 2.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_secretsmanager_secret.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/secretsmanager_secret) | resource |
| [aws_secretsmanager_secret_version.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/secretsmanager_secret_version) | resource |
| [kubectl_manifest.secret_provider_class](https://registry.terraform.io/providers/alekc/kubectl/latest/docs/resources/manifest) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_create_secret_provider_class"></a> [create\_secret\_provider\_class](#input\_create\_secret\_provider\_class) | When true, render the Secrets Store CSI SecretProviderClass so pods can mount the secrets. Set false in greenfield Phase 1 (before the CSI CRD exists on the cluster) — the secrets are still created. | `bool` | `true` | no |
| <a name="input_kms_key_arn"></a> [kms\_key\_arn](#input\_kms\_key\_arn) | Resolved CMK ARN (from the kms module). Secret material is envelope-encrypted with THIS key. | `string` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for the secrets (also the path prefix the iam runtime policy scopes to). | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the SecretProviderClass is created. | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | AWS region the secrets live in. | `string` | n/a | yes |
| <a name="input_secrets"></a> [secrets](#input\_secrets) | Map of logical name => initial secret string value. Keys build the secret names; values are the secret material, written to KMS-encrypted Secrets Manager secrets. Rotate out-of-band afterward. | `map(string)` | `{}` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags applied to the secrets. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_secret_arns"></a> [secret\_arns](#output\_secret\_arns) | ARNs of the created secrets. Recorded for review; the iam runtime policy scopes GetSecretValue at the name path prefix, not these ARNs. |
| <a name="output_secret_provider_class_name"></a> [secret\_provider\_class\_name](#output\_secret\_provider\_class\_name) | Name of the rendered Secrets Store CSI SecretProviderClass (empty when disabled). |
| <a name="output_secrets_ref"></a> [secrets\_ref](#output\_secrets\_ref) | Mounting reference: the SecretProviderClass name the workload pod mounts (empty when disabled). |
<!-- END_TF_DOCS -->
