# `workload` module

Deploys the workload from a single YAML spec, in whichever install tier preflight selected.

- Takes one `spec_yaml` document — the single source of the workload's shape (image, port,
  autoscale, probes, resources, security contexts, …). Both tiers derive from it, so the Tier A
  Workload CR spec and the Tier B chart values cannot drift.
- **Tier A** (`install_tier = "A"`): applies a `Workload` custom resource via `kubectl_manifest`
  (raw YAML — no plan-time CRD schema discovery, so it plans before the CRD exists and applies in
  the same run after the operator installs it). A gated, time-bounded `wait_for` blocks on the
  operator-set `Ready` status condition.
- **Tier B** (`install_tier = "B"`): renders `charts/workload` directly via `helm_release`, passing
  the spec as helm `values`.

`wait_for_ready` (default `true`) can be disabled on rate-limited clusters; readiness is then
confirmed out-of-band.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_helm"></a> [helm](#requirement\_helm) | ~> 2.13 |
| <a name="requirement_kubectl"></a> [kubectl](#requirement\_kubectl) | ~> 2.0 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.30 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_helm"></a> [helm](#provider\_helm) | ~> 2.13 |
| <a name="provider_kubectl"></a> [kubectl](#provider\_kubectl) | ~> 2.0 |
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [helm_release.workload](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [kubectl_manifest.workload_cr](https://registry.terraform.io/providers/alekc/kubectl/latest/docs/resources/manifest) | resource |
| [terraform_data.ordering](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_crd_ready"></a> [crd\_ready](#input\_crd\_ready) | Ordering handle: the k8s-platform crd\_established id (Tier A). Referencing it makes the Workload CR apply only after the Workload CRD is Established, removing the CRD-vs-CR race without a root-level depends\_on. | `string` | `""` | no |
| <a name="input_install_tier"></a> [install\_tier](#input\_install\_tier) | "A" applies a Workload CR (operator reconciles). "B" renders charts/workload directly via helm\_release. | `string` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Workload name (CRD metadata.name / charts/workload .Values.name). | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Target namespace. | `string` | n/a | yes |
| <a name="input_pdb_min_available"></a> [pdb\_min\_available](#input\_pdb\_min\_available) | PodDisruptionBudget minAvailable (charts/workload .Values.pdb.minAvailable). Tier B only (the chart owns the PDB; the CRD does not expose it). | `number` | `1` | no |
| <a name="input_secrets_ready"></a> [secrets\_ready](#input\_secrets\_ready) | Ordering handle: a value derived from the secret material (e.g. joined secret ids). Referencing it makes the Workload apply only after the secrets exist, so a pod mounting them via the CSI SecretProviderClass does not start before the material is present. | `string` | `""` | no |
| <a name="input_spec_yaml"></a> [spec\_yaml](#input\_spec\_yaml) | The Workload spec as YAML — the SINGLE source of the workload's shape for both tiers. Its<br/>fields match the Workload CRD spec and charts/workload values.schema.json (minus name/namespace,<br/>which are identity/wiring): image, port, autoscale{minReplicas,maxReplicas,targetCPUUtilization},<br/>and optionally livenessProbe{path,port}, readinessProbe{path,port}, resources,<br/>securityContext, podSecurityContext, rolloutStrategy, ingressClass, ingress.<br/><br/>Tier A wraps this verbatim as the Workload CR's `spec`; Tier B passes it as helm values<br/>(merged with name/namespace/pdb). One document, so the CR spec and the chart values cannot<br/>drift. Per-cloud values are supplied by composing/merging YAML at the root. | `string` | n/a | yes |
| <a name="input_wait_for_ready"></a> [wait\_for\_ready](#input\_wait\_for\_ready) | Tier A only: block the apply until the operator sets the Workload Ready=True. Disable on rate-limited/slow clusters where the readiness poll flakes; readiness can then be confirmed out-of-band (kubectl wait). | `bool` | `true` | no |
| <a name="input_wait_timeout"></a> [wait\_timeout](#input\_wait\_timeout) | Tier A only: timeout for the create/readiness wait on the Workload CR (e.g. "5m"). | `string` | `"5m"` | no |
| <a name="input_workload_chart_path"></a> [workload\_chart\_path](#input\_workload\_chart\_path) | Path to the shared workload Helm chart (charts/workload). Tier B only. | `string` | `"../../charts/workload"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_namespace"></a> [namespace](#output\_namespace) | The workload namespace. |
| <a name="output_tier"></a> [tier](#output\_tier) | The active install tier. |
| <a name="output_workload_name"></a> [workload\_name](#output\_workload\_name) | Name of the Workload (CR in Tier A, helm release in Tier B). |
<!-- END_TF_DOCS -->
