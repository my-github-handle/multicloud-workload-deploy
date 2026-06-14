# `preflight` module

Runs the preflight checker binary through a hashicorp `external` data source and turns its verdict
into a deploy gate.

- Invokes the binary with `--mode=agnostic --kubeconfig <path> --namespace <ns>` and reads its flat
  `{verdict, report_json}` output.
- **Hard-gates a red verdict** on the data source's `postcondition` (evaluated at plan time, on the
  critical path of every consumer) so a red result fails the plan before any resource is created.
  Controlled by `fail_on_red` (default `true`).
- Surfaces an **amber** verdict as a non-blocking `check` warning carrying the full report.
- **Derives the install tier** (`A` operator / `B` namespaced) from the `k8s.installtier` result;
  an identity that cannot deploy at all yields `RED`, which trips downstream validation rather than
  half-deploying. `install_tier_override` forces a tier.

Outputs `verdict`, the decoded `report`, and `install_tier` for the downstream modules.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_external"></a> [external](#requirement\_external) | ~> 2.3 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_external"></a> [external](#provider\_external) | ~> 2.3 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [external_external.preflight](https://registry.terraform.io/providers/hashicorp/external/latest/docs/data-sources/external) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_fail_on_red"></a> [fail\_on\_red](#input\_fail\_on\_red) | When true, a red verdict fails the plan via the data-source postcondition. The check block always reports; this flag controls hard-blocking. | `bool` | `true` | no |
| <a name="input_install_tier_override"></a> [install\_tier\_override](#input\_install\_tier\_override) | Explicit install tier override: "A", "B", or "" (empty = derive from the preflight report's k8s.installtier result). | `string` | `""` | no |
| <a name="input_kubeconfig_path"></a> [kubeconfig\_path](#input\_kubeconfig\_path) | Path to the kubeconfig the preflight binary uses for the real Kubernetes stages 4-5. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Target workload namespace passed to the preflight binary (--namespace). | `string` | n/a | yes |
| <a name="input_preflight_binary"></a> [preflight\_binary](#input\_preflight\_binary) | Absolute path to the preflight checker binary (operator/cmd/preflight build output). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_install_tier"></a> [install\_tier](#output\_install\_tier) | Derived install tier: "A" (operator) or "B" (namespaced manifests). Override via install\_tier\_override. ("RED" is surfaced only when fail\_on\_red is disabled and the identity cannot deploy at all — it deliberately trips downstream validation.) |
| <a name="output_report"></a> [report](#output\_report) | The full decoded preflight Report (stages + results). |
| <a name="output_verdict"></a> [verdict](#output\_verdict) | Top-level preflight verdict: green \| amber \| red. |
<!-- END_TF_DOCS -->
