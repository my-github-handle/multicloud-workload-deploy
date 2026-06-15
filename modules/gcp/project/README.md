# `project` module (GCP)

Resolves the GCP **project** the deployment lives in, folding the create-vs-lookup decision into
this single module. On GCP the project is the fundamental container — billing link, IAM boundary,
and the scope service APIs are enabled on.

- **provision mode** creates a dedicated `google_project` (under an org or folder, linked to a
  billing account), disables the default permissive network, and enables the required service APIs.
- **byo mode** resolves an existing customer project and still ensures the required APIs are
  enabled (the BYOC baseline — customers commonly pre-create the project).

The resolved `project_id` / `project_number` are consumed by every downstream module, so create vs
BYO is a single switch.

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
| [google_project.this](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/project) | resource |
| [google_project_service.required](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/project_service) | resource |
| [google_project.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/project) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_activate_apis"></a> [activate\_apis](#input\_activate\_apis) | Service APIs to enable on the project. These are the APIs every building block in this product needs (GKE, Compute, Cloud KMS, Secret Manager, IAM + credentials, Logging, Monitoring, Artifact Registry, Resource Manager, Service Usage). Enabled in BOTH modes so a BYOC project is brought up to the required baseline. | `list(string)` | <pre>[<br/>  "compute.googleapis.com",<br/>  "container.googleapis.com",<br/>  "cloudkms.googleapis.com",<br/>  "secretmanager.googleapis.com",<br/>  "iam.googleapis.com",<br/>  "iamcredentials.googleapis.com",<br/>  "logging.googleapis.com",<br/>  "monitoring.googleapis.com",<br/>  "artifactregistry.googleapis.com",<br/>  "cloudresourcemanager.googleapis.com",<br/>  "serviceusage.googleapis.com",<br/>  "dns.googleapis.com"<br/>]</pre> | no |
| <a name="input_billing_account"></a> [billing\_account](#input\_billing\_account) | Billing account id to link the created project to (provision mode), e.g. "0X0X0X-0X0X0X-0X0X0X". Required in provision mode; ignored in byo mode. | `string` | `""` | no |
| <a name="input_disable_services_on_destroy"></a> [disable\_services\_on\_destroy](#input\_disable\_services\_on\_destroy) | Whether `terraform destroy` disables the enabled APIs. Default false: on a BYO project we must NOT disable APIs the customer may rely on elsewhere; on a provisioned project the whole project is destroyed anyway. | `bool` | `false` | no |
| <a name="input_folder_id"></a> [folder\_id](#input\_folder\_id) | Folder id to create the project under (provision mode). Mutually exclusive with org\_id. | `string` | `""` | no |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels applied to the provisioned project. | `map(string)` | `{}` | no |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" creates a new project and enables the required service APIs; "byo" resolves an existing customer project and ensures the same APIs are enabled (the BYOC path — customers commonly pre-create the project). | `string` | n/a | yes |
| <a name="input_org_id"></a> [org\_id](#input\_org\_id) | Organization id to create the project under (provision mode). Mutually exclusive with folder\_id; leave both empty for a project with no parent. | `string` | `""` | no |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | Project ID. In provision mode this is the ID for the project to create; in byo mode the existing project to resolve. | `string` | n/a | yes |
| <a name="input_project_name"></a> [project\_name](#input\_project\_name) | Human-readable project display name (provision mode). Defaults to project\_id when empty. | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_enabled_services"></a> [enabled\_services](#output\_enabled\_services) | The set of service APIs ensured enabled on the project. |
| <a name="output_project_id"></a> [project\_id](#output\_project\_id) | Resolved project ID (created or looked up) — identical shape in both modes. Every downstream module consumes this so create-vs-BYO is a single switch. |
| <a name="output_project_number"></a> [project\_number](#output\_project\_number) | Resolved project number (created or looked up). |
<!-- END_TF_DOCS -->
