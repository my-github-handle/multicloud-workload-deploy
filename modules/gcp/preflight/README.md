# GCP preflight: the two halves

The GCP preflight contract has two co-located parts, by design:

1. **Go `cloud.Provider` (the real staged cloud checks).**
   `operator/internal/cloud/gcp` implements the four `cloud.Provider` methods —
   `CheckIdentityPermissions` (Stage 0), `CheckKMSKey` (Stage 1),
   `CheckSecretsBackend` (Stage 2), `CheckEgress` (Stage 3) — using the
   `cloud.google.com/go/*` SDKs + `google.golang.org/api`. The preflight binary
   (`operator/cmd/preflight`) selects this provider via `--cloud=gcp`
   (`selectProvider("gcp")`), runs the staged Runner, and emits the
   green/amber/red report. The plan-C `modules/preflight` invokes the binary and
   gates `terraform apply` on the verdict. **This is where the permission
   simulation (IAM testIamPermissions), KMS key/version state, secrets-CMEK, and
   egress-posture checks live.**

2. **This Terraform module (co-located data-source sanity pre-checks).**
   `checks.tf` asserts, inside the plan graph, that the *resolved* resources are
   coherent: the project resolves, the VPC network resolves, the CryptoKey
   resolves. These are cheap precondition guards that fail the plan fast and live
   next to the modules that produce the inputs. They do **not** re-implement the
   staged report — they backstop it with graph-time assertions.

In `gcp-full`, the binary runs with `--mode=full --cloud=gcp`: stages the
greenfield path satisfies *by provisioning* are informational (downgraded
red→amber), not blocking. This module's preconditions still hard-fail on a
genuinely unresolvable resolved resource.

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
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [terraform_data.kms_resolves](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.network_resolves](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.project_resolves](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [google_compute_network.resolved](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_network) | data source |
| [google_kms_crypto_key.resolved](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/kms_crypto_key) | data source |
| [google_project.current](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/project) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_kms_key_id"></a> [kms\_key\_id](#input\_kms\_key\_id) | Resolved CryptoKey id to sanity-check (must resolve). | `string` | n/a | yes |
| <a name="input_network_self_link"></a> [network\_self\_link](#input\_network\_self\_link) | Resolved VPC network self-link to sanity-check (must resolve). | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID (asserted to resolve). | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | GCP region (recorded in the report/logs). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_checks_passed"></a> [checks\_passed](#output\_checks\_passed) | True once all co-located GCP data-source preconditions have evaluated (project/network/KMS). |
| <a name="output_project_number"></a> [project\_number](#output\_project\_number) | The GCP project number the deploy is running against (for the report/logs). |
| <a name="output_region"></a> [region](#output\_region) | The GCP region the deploy is running against (echoed for the report/logs). |
<!-- END_TF_DOCS -->
