# `kms` module (GCP)

Provides the customer-managed Cloud KMS CryptoKey used for envelope encryption across the stack —
Secret Manager material and GKE database/application-layer secrets at rest — folding the
create-vs-lookup decision into this single module.

- **provision mode** creates a KeyRing + CryptoKey with an automatic rotation period and
  `prevent_destroy` (KMS keys cannot be truly deleted, only scheduled for destruction).
- **byo mode** resolves a customer-supplied CryptoKey resource id (validated as the canonical
  `projects/P/locations/L/keyRings/R/cryptoKeys/K` form).

The resolved `key_id` is the single reference consumed by the `iam`, `secrets`, and `cluster`
modules, so every consumer encrypts under the same key.

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
| [google_kms_crypto_key.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/kms_crypto_key) | resource |
| [google_kms_key_ring.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/kms_key_ring) | resource |
| [google_kms_crypto_key.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/kms_crypto_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_crypto_key_name"></a> [crypto\_key\_name](#input\_crypto\_key\_name) | CryptoKey name (provision mode). | `string` | `"workload-cmk"` | no |
| <a name="input_key_ring_name"></a> [key\_ring\_name](#input\_key\_ring\_name) | KeyRing name (provision mode). | `string` | `"workload-keyring"` | no |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels applied to the provisioned CryptoKey. | `map(string)` | `{}` | no |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" creates a new KeyRing + CryptoKey with rotation; "byo" resolves a customer-supplied CryptoKey id. | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID the key lives in. | `string` | n/a | yes |
| <a name="input_provided_key_id"></a> [provided\_key\_id](#input\_provided\_key\_id) | Existing CryptoKey resource id to resolve (byo mode), e.g. projects/P/locations/L/keyRings/R/cryptoKeys/K. Ignored in provision mode. | `string` | `""` | no |
| <a name="input_region"></a> [region](#input\_region) | Location (region) of the KeyRing (provision mode). Should match the cluster/secret region. | `string` | `"us-central1"` | no |
| <a name="input_rotation_period"></a> [rotation\_period](#input\_rotation\_period) | Automatic rotation period for the CryptoKey (provision mode), e.g. "7776000s" (90 days). | `string` | `"7776000s"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_crypto_key_name"></a> [crypto\_key\_name](#output\_crypto\_key\_name) | CryptoKey short name in provision mode; empty in BYO mode. |
| <a name="output_key_id"></a> [key\_id](#output\_key\_id) | Resolved CryptoKey resource id (created or BYO), e.g. projects/P/locations/L/keyRings/R/cryptoKeys/K. |
| <a name="output_key_ring_id"></a> [key\_ring\_id](#output\_key\_ring\_id) | KeyRing id in provision mode; empty in BYO mode. |
<!-- END_TF_DOCS -->
