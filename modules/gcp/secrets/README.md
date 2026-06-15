# `secrets` module (GCP)

Creates CMEK-encrypted Secret Manager secrets and wires the Secrets Store CSI driver so workload
pods mount them at runtime via Workload Identity.

- Each secret uses a user-managed regional replica encrypted with the resolved Cloud KMS key
  (CMEK), pinned to the key's region.
- The `SecretProviderClass` is applied as raw YAML via `kubectl_manifest` (no plan-time CRD schema
  discovery), so the module plans offline and applies once the GKE Secret Manager CSI add-on's CRD
  is present. Gated by `create_secret_provider_class`.

`secrets_ref` (the SecretProviderClass name) is what the workload pod mounts.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_google"></a> [google](#requirement\_google) | ~> 6.0 |
| <a name="requirement_kubectl"></a> [kubectl](#requirement\_kubectl) | ~> 2.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | ~> 6.0 |
| <a name="provider_kubectl"></a> [kubectl](#provider\_kubectl) | ~> 2.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [google_secret_manager_secret.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/secret_manager_secret) | resource |
| [google_secret_manager_secret_version.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/secret_manager_secret_version) | resource |
| [kubectl_manifest.secret_provider_class](https://registry.terraform.io/providers/alekc/kubectl/latest/docs/resources/manifest) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_create_secret_provider_class"></a> [create\_secret\_provider\_class](#input\_create\_secret\_provider\_class) | When true, render the Secrets Store CSI SecretProviderClass so pods can mount the secrets. | `bool` | `true` | no |
| <a name="input_kms_key_id"></a> [kms\_key\_id](#input\_kms\_key\_id) | Resolved CryptoKey id (from kms module). Secret material is CMEK-encrypted with THIS key. | `string` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for the secrets. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the SecretProviderClass is created. | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID the secrets live in. | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | Region for the Secret Manager replica (CMEK requires a regional replica matching the key location). | `string` | `"us-central1"` | no |
| <a name="input_secrets"></a> [secrets](#input\_secrets) | Map of logical name => initial secret string value. Stored CMEK-encrypted; rotate out-of-band afterward. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_secret_ids"></a> [secret\_ids](#output\_secret\_ids) | Resource ids of the created secrets — fed into the iam runtime bindings (scoped versions.access). |
| <a name="output_secret_provider_class_name"></a> [secret\_provider\_class\_name](#output\_secret\_provider\_class\_name) | Name of the rendered Secrets Store CSI SecretProviderClass (empty when disabled). |
| <a name="output_secrets_ref"></a> [secrets\_ref](#output\_secrets\_ref) | Mounting reference: the SecretProviderClass name the workload pod mounts. |
<!-- END_TF_DOCS -->
