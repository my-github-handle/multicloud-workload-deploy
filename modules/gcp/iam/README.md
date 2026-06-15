# `iam` module (GCP)

Provisions the workload's runtime identity and renders the least-privilege policies as reviewable
artifacts.

- **provision mode** creates a Google service account (GSA), a wildcard-free custom role (no
  primitive owner/editor/viewer roles), resource-scoped bindings (cloudkms on the resolved key,
  artifactregistry.reader on the resolved repos, `secretmanager.versions.access` scoped by an IAM
  condition on the secret-name prefix), and the Workload Identity binding of the Kubernetes SA to
  the GSA (`roles/iam.workloadIdentityUser` on `serviceAccount:PROJECT.svc.id.goog[NS/KSA]`).
- **byo mode** resolves a customer-supplied GSA and still emits the role + WI-binding docs to
  attach.

The secrets binding is scoped by a deterministic name **prefix**, so this module never consumes the
`secrets` module's output ids — breaking the iam↔secrets cycle. Both the runtime and the
deploy-time policies are rendered to `artifacts/` at apply time (generated, not checked in) and
asserted wildcard-free / no-primitive-role by `tests/no_wildcards.tftest.hcl`.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_google"></a> [google](#requirement\_google) | ~> 6.0 |
| <a name="requirement_local"></a> [local](#requirement\_local) | ~> 2.5 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | ~> 6.0 |
| <a name="provider_local"></a> [local](#provider\_local) | ~> 2.5 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [google_artifact_registry_repository_iam_member.runtime_repos](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/artifact_registry_repository_iam_member) | resource |
| [google_kms_crypto_key_iam_member.runtime_kms](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/kms_crypto_key_iam_member) | resource |
| [google_project_iam_custom_role.runtime](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/project_iam_custom_role) | resource |
| [google_project_iam_member.runtime_secrets](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/project_iam_member) | resource |
| [google_service_account.workload](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/service_account) | resource |
| [google_service_account_iam_member.wi_user](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/service_account_iam_member) | resource |
| [local_file.bindings](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.custom_role](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.deploy_role](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.wi_binding](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [google_service_account.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/service_account) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_artifact_registry_repo_ids"></a> [artifact\_registry\_repo\_ids](#input\_artifact\_registry\_repo\_ids) | Artifact Registry repository ids the workload pulls from. Runtime binding scopes artifactregistry.reader to these repos only. | `list(string)` | `[]` | no |
| <a name="input_artifacts_dir"></a> [artifacts\_dir](#input\_artifacts\_dir) | Directory to write the reviewable role/binding JSON artifacts into. | `string` | `"artifacts"` | no |
| <a name="input_k8s_service_account"></a> [k8s\_service\_account](#input\_k8s\_service\_account) | Kubernetes ServiceAccount name bound to the GSA via Workload Identity (workload + connect-agent). | `string` | `"workload"` | no |
| <a name="input_kms_key_id"></a> [kms\_key\_id](#input\_kms\_key\_id) | Resolved CryptoKey id (from kms module). Runtime binding scopes cloudkms encrypt/decrypt to THIS key only — no project-wide grant. | `string` | n/a | yes |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" creates the GSA, custom role, bindings, and Workload Identity binding; "byo" emits the rendered role + WI-binding docs and resolves a customer-supplied GSA email. | `string` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Name prefix for the service account and custom role. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace of the workload + connect-agent ServiceAccount. | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID — bindings are scoped to resolved resources within this project (no project-wide primitive roles). | `string` | n/a | yes |
| <a name="input_provided_gsa_email"></a> [provided\_gsa\_email](#input\_provided\_gsa\_email) | Existing Google service account email to resolve (byo-identity mode). The module still emits the role + WI-binding docs for the customer to attach. | `string` | `""` | no |
| <a name="input_secret_name_prefix"></a> [secret\_name\_prefix](#input\_secret\_name\_prefix) | Deterministic Secret Manager secret-name prefix the workload's secrets share (e.g. "<name>-"). The runtime versions.access binding is scoped to secrets matching this prefix via an IAM condition — NO dependency on the secrets module (breaks the iam↔secrets cycle). | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_custom_role_id"></a> [custom\_role\_id](#output\_custom\_role\_id) | Resolved custom role id (provision mode); empty in byo mode. |
| <a name="output_custom_role_json"></a> [custom\_role\_json](#output\_custom\_role\_json) | Rendered RUNTIME least-privilege custom role document — enumerated permissions only, NO primitive roles, NO wildcards (reviewable artifact). |
| <a name="output_deploy_role_json"></a> [deploy\_role\_json](#output\_deploy\_role\_json) | Rendered DEPLOY-TIME least-privilege custom role document — create/manage permissions for the gcp-full path, NO primitive roles, NO wildcards. Asserted consistent with the Go requiredDeployPermissions probe. |
| <a name="output_gsa_email"></a> [gsa\_email](#output\_gsa\_email) | Resolved Google service account email (created in provision mode, looked up in byo-identity mode). |
| <a name="output_ksa_annotation"></a> [ksa\_annotation](#output\_ksa\_annotation) | The annotation to put on the Kubernetes ServiceAccount so GKE Workload Identity maps it to the GSA. |
| <a name="output_wi_member"></a> [wi\_member](#output\_wi\_member) | The Workload Identity pool member: serviceAccount:PROJECT.svc.id.goog[NS/KSA]. |
| <a name="output_workload_identity_ref"></a> [workload\_identity\_ref](#output\_workload\_identity\_ref) | Workload identity reference — the GSA email the KSA annotates with iam.gke.io/gcp-service-account. |
<!-- END_TF_DOCS -->
