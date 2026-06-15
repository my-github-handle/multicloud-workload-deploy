# `cluster-resolver` module (GCP)

The create-vs-lookup branch point for the GKE cluster. Emits a uniform `{endpoint, ca, auth}`
interface — identical shape both modes — consumed by the root's `kubernetes`/`helm` providers.

- **provision mode** passes the `cluster` module's endpoint/CA through.
- **byo mode** looks up an existing cluster via `data.google_container_cluster`.

`auth` is a short-lived Google access token (`data.google_client_config`) — the GKE token-auth
model. The endpoint is scheme-normalized to a single `https://` prefix in both modes.

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
| [google_client_config.current](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/client_config) | data source |
| [google_container_cluster.byo](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/container_cluster) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cluster_name"></a> [cluster\_name](#input\_cluster\_name) | GKE cluster name. In provision mode the created cluster's name; in byo mode the existing cluster to look up. | `string` | n/a | yes |
| <a name="input_location"></a> [location](#input\_location) | Cluster location (region/zone). In provision mode the created cluster's location; in byo mode the existing cluster's location. | `string` | n/a | yes |
| <a name="input_mode"></a> [mode](#input\_mode) | "provision" feeds the cluster module outputs through; "byo" looks up an existing GKE cluster via data sources. | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID (BYO lookup). | `string` | n/a | yes |
| <a name="input_provisioned_ca"></a> [provisioned\_ca](#input\_provisioned\_ca) | Cluster CA data from the cluster module (provision mode). | `string` | `""` | no |
| <a name="input_provisioned_endpoint"></a> [provisioned\_endpoint](#input\_provisioned\_endpoint) | Cluster endpoint from the cluster module (provision mode). | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_auth"></a> [auth](#output\_auth) | Short-lived Google access token for the kubernetes/helm providers (the {endpoint, ca, auth} uniform interface). GKE token auth. |
| <a name="output_ca"></a> [ca](#output\_ca) | Resolved base64 cluster CA data. |
| <a name="output_endpoint"></a> [endpoint](#output\_endpoint) | Resolved cluster API endpoint (created or looked up), https:// prefixed — identical shape both modes. |
<!-- END_TF_DOCS -->
