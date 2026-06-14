# `k8s-platform` module

Installs the install-tier platform layer.

- **Tier A** (`install_tier = "A"`): installs the `workload-operator` Helm chart — the Workload
  CRD, the namespace-scoped controller Deployment + RBAC, and (optionally) creates the namespace.
  The release waits for the controller to be ready.
- **Tier B** (`install_tier = "B"`): a no-op (the `workload` module renders `charts/workload`
  directly).
- **CRD-Established gate** (Tier A): after the operator install, a `terraform_data` runs
  `kubectl wait --for=condition=established` on the Workload CRD. `helm_release wait=true` settles
  only the chart's resources, not the api-server's CRD registration; this gate closes the race so a
  same-apply Workload CR is not rejected with "no matches for kind Workload". Exposed via the
  `crd_established` output for the workload module to depend on.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_helm"></a> [helm](#requirement\_helm) | ~> 2.13 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.30 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_helm"></a> [helm](#provider\_helm) | ~> 2.13 |
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [helm_release.operator](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [terraform_data.crd_established](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_crd_name"></a> [crd\_name](#input\_crd\_name) | Name of the Workload CRD to wait for Established (Tier A). | `string` | `"workloads.workload.ops.dev"` | no |
| <a name="input_crd_wait_timeout"></a> [crd\_wait\_timeout](#input\_crd\_wait\_timeout) | Timeout for the `kubectl wait --for=condition=established` gate on the Workload CRD (Tier A). | `string` | `"120s"` | no |
| <a name="input_create_namespace"></a> [create\_namespace](#input\_create\_namespace) | Whether the operator chart release should create the namespace. | `bool` | `true` | no |
| <a name="input_install_tier"></a> [install\_tier](#input\_install\_tier) | "A" installs the workload-operator chart (CRD + namespace-scoped RBAC + controller). "B" is a no-op (workload module renders charts/workload directly). | `string` | n/a | yes |
| <a name="input_kube_context"></a> [kube\_context](#input\_kube\_context) | Optional kubeconfig context for the CRD-Established wait. Empty uses the current-context. | `string` | `""` | no |
| <a name="input_kubeconfig_path"></a> [kubeconfig\_path](#input\_kubeconfig\_path) | Path to the kubeconfig, used by the CRD-Established wait (Tier A). Must match the providers' kubeconfig. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Namespace the operator is installed into and (in Tier A) watches. | `string` | n/a | yes |
| <a name="input_operator_chart_path"></a> [operator\_chart\_path](#input\_operator\_chart\_path) | Path to the workload-operator Helm chart (charts/workload-operator). | `string` | `"../../charts/workload-operator"` | no |
| <a name="input_operator_image_repository"></a> [operator\_image\_repository](#input\_operator\_image\_repository) | Operator controller image repository. | `string` | `"ghcr.io/ops-dev/workload-operator"` | no |
| <a name="input_operator_image_tag"></a> [operator\_image\_tag](#input\_operator\_image\_tag) | Operator controller image tag. | `string` | `"0.1.0"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_crd_established"></a> [crd\_established](#output\_crd\_established) | Ordering handle: resolves only after the Workload CRD is Established (Tier A). The workload module depends on this so the Tier A Workload CR applies after the CRD is registered. Empty in Tier B. |
| <a name="output_namespace"></a> [namespace](#output\_namespace) | The namespace the platform layer targets (passed through for downstream wiring). |
| <a name="output_operator_release_name"></a> [operator\_release\_name](#output\_operator\_release\_name) | Helm release name of the operator in Tier A; empty string in Tier B. |
| <a name="output_tier"></a> [tier](#output\_tier) | The active install tier. |
<!-- END_TF_DOCS -->
