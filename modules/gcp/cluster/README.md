# `cluster` module (GCP)

Provisions a hardened, private GKE cluster via the maintained `terraform-google-modules`
beta-private-cluster module (pinned `~> 33.0`).

- **Private** nodes (no public IPs) and a private control-plane endpoint; a testing-only toggle
  (`enable_private_endpoint = false` + `master_authorized_networks`) exposes the endpoint to a CIDR
  allowlist.
- **Dataplane V2 (= Cilium)** so Cilium and Kubernetes `NetworkPolicy` are native — no separate
  Cilium install — with Hubble-grade observability enabled on the cluster.
- **Workload Identity** + metadata concealment (pods reach the GKE metadata server, never the raw
  VM metadata endpoint), shielded nodes, CMEK database/application-layer secrets encryption with
  the resolved key, a release channel, and full control-plane logging/monitoring.

Outputs the `{cluster_name, endpoint, ca, location, workload_identity_pool}` the cluster-resolver
re-exports uniformly.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_google"></a> [google](#requirement\_google) | ~> 6.0 |
| <a name="requirement_google-beta"></a> [google-beta](#requirement\_google-beta) | ~> 6.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | ~> 6.0 |
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_gke"></a> [gke](#module\_gke) | terraform-google-modules/kubernetes-engine/google//modules/beta-private-cluster | ~> 33.0 |

## Resources

| Name | Type |
|------|------|
| [google_kms_crypto_key_iam_member.gke_robot](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/kms_crypto_key_iam_member) | resource |
| [terraform_data.node_bounds](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_enable_private_endpoint"></a> [enable\_private\_endpoint](#input\_enable\_private\_endpoint) | When true (default), the control-plane endpoint is private (in-VPC only). Set false ONLY for testing to reach the API server from outside the VPC; pair with master\_authorized\_networks to restrict access to a CIDR allowlist. | `bool` | `true` | no |
| <a name="input_enable_secret_manager_csi_addon"></a> [enable\_secret\_manager\_csi\_addon](#input\_enable\_secret\_manager\_csi\_addon) | Enable the GKE-managed Secret Manager CSI driver add-on. The secrets module's SecretProviderClass apply FAILS on a clean greenfield cluster unless the Secrets-Store-CSI driver + GCP provider are present. Enabling this managed add-on installs them so the SPC mounts; if false, the consumer must install the CSI driver out-of-band (or set secrets.create\_secret\_provider\_class=false). | `bool` | `true` | no |
| <a name="input_k8s_version"></a> [k8s\_version](#input\_k8s\_version) | Kubernetes minimum master version / node version (or leave to the release channel). | `string` | `"1.30"` | no |
| <a name="input_kms_key_id"></a> [kms\_key\_id](#input\_kms\_key\_id) | Resolved CryptoKey id (from kms module) for GKE database/application-layer secrets encryption at rest. | `string` | n/a | yes |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels applied to the cluster. | `map(string)` | `{}` | no |
| <a name="input_master_authorized_networks"></a> [master\_authorized\_networks](#input\_master\_authorized\_networks) | CIDR blocks allowed to reach the control-plane endpoint, as a list of { cidr\_block, display\_name }. Empty by default. When enable\_private\_endpoint=false (testing), set this to a tight allowlist so the public endpoint is not open to the world. Supply the CIDR at apply time; do not commit it. | <pre>list(object({<br/>    cidr_block   = string<br/>    display_name = string<br/>  }))</pre> | `[]` | no |
| <a name="input_master_ipv4_cidr_block"></a> [master\_ipv4\_cidr\_block](#input\_master\_ipv4\_cidr\_block) | The /28 CIDR for the private control-plane endpoint. | `string` | `"172.16.0.0/28"` | no |
| <a name="input_name"></a> [name](#input\_name) | GKE cluster name. | `string` | n/a | yes |
| <a name="input_network_self_link"></a> [network\_self\_link](#input\_network\_self\_link) | Resolved VPC network self-link (from network-resolver). | `string` | n/a | yes |
| <a name="input_node_machine_type"></a> [node\_machine\_type](#input\_node\_machine\_type) | Node pool machine type. | `string` | `"e2-standard-4"` | no |
| <a name="input_node_max_count"></a> [node\_max\_count](#input\_node\_max\_count) | Maximum nodes per zone. | `number` | `3` | no |
| <a name="input_node_min_count"></a> [node\_min\_count](#input\_node\_min\_count) | Minimum nodes per zone. | `number` | `1` | no |
| <a name="input_pods_range_name"></a> [pods\_range\_name](#input\_pods\_range\_name) | Secondary range name for pods (alias IPs). | `string` | n/a | yes |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID. | `string` | n/a | yes |
| <a name="input_project_number"></a> [project\_number](#input\_project\_number) | GCP project number (from the project module). Used to construct the GKE service-agent member without re-reading the project. | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | GKE cluster location (region for a regional cluster). | `string` | `"us-central1"` | no |
| <a name="input_release_channel"></a> [release\_channel](#input\_release\_channel) | GKE release channel: RAPID \| REGULAR \| STABLE. | `string` | `"REGULAR"` | no |
| <a name="input_services_range_name"></a> [services\_range\_name](#input\_services\_range\_name) | Secondary range name for services (alias IPs). | `string` | n/a | yes |
| <a name="input_subnet_self_link"></a> [subnet\_self\_link](#input\_subnet\_self\_link) | Resolved node subnet self-link (from network-resolver / network module). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_ca"></a> [ca](#output\_ca) | Base64-encoded cluster CA certificate. |
| <a name="output_cluster_name"></a> [cluster\_name](#output\_cluster\_name) | GKE cluster name. |
| <a name="output_endpoint"></a> [endpoint](#output\_endpoint) | GKE API server endpoint (private by default; public when enable\_private\_endpoint = false). |
| <a name="output_location"></a> [location](#output\_location) | Cluster location (region) — consumed by the cluster-resolver. |
| <a name="output_workload_identity_pool"></a> [workload\_identity\_pool](#output\_workload\_identity\_pool) | The Workload Identity pool (PROJECT.svc.id.goog) the iam module binds the KSA through. |
<!-- END_TF_DOCS -->
