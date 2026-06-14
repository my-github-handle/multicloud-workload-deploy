# `_agnostic-deploy` root

The BYOC fast-path entry point: **one `terraform apply`** against an existing Kubernetes cluster
(kubeconfig only, no cloud-provider credentials) produces a preflight-gated, secure, observable,
lifecycle-managed Workload.

It composes the five Layer-3 modules in order:

1. **preflight** — gate on the verdict (red blocks the plan) and derive the install tier.
2. **k8s-platform** — Tier A installs the operator + CRD (with the CRD-Established wait); Tier B
   is a no-op.
3. **k8s-security** — default-deny + metadata-block NetworkPolicies and the configurable
   PodSecurity floor.
4. **k8s-observability** — in-cluster ServiceMonitor + Grafana dashboard.
5. **workload** — Tier A Workload CR / Tier B `charts/workload`, from a single YAML spec.

Providers (`kubernetes`/`helm`/`kubectl`/`external`) are configured from the kubeconfig only —
there is **no cloud provider block**. The workload is supplied as one `workload_spec_yaml`
document; the namespace's allowed workload port is derived from it so they cannot drift. See
[`verify-on-kind.md`](./verify-on-kind.md) for an end-to-end walkthrough, and
[`../../docs/operations/`](../../docs/operations) for the operator guides.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_external"></a> [external](#requirement\_external) | ~> 2.3 |
| <a name="requirement_helm"></a> [helm](#requirement\_helm) | ~> 2.13 |
| <a name="requirement_kubectl"></a> [kubectl](#requirement\_kubectl) | ~> 2.0 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.30 |

## Providers

No providers.

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_k8s_observability"></a> [k8s\_observability](#module\_k8s\_observability) | ../../modules/k8s-observability | n/a |
| <a name="module_k8s_platform"></a> [k8s\_platform](#module\_k8s\_platform) | ../../modules/k8s-platform | n/a |
| <a name="module_k8s_security"></a> [k8s\_security](#module\_k8s\_security) | ../../modules/k8s-security | n/a |
| <a name="module_preflight"></a> [preflight](#module\_preflight) | ../../modules/preflight | n/a |
| <a name="module_workload"></a> [workload](#module\_workload) | ../../modules/workload | n/a |

## Resources

No resources.

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_control_plane_port"></a> [control\_plane\_port](#input\_control\_plane\_port) | Control-plane egress port (the in-cluster egress-allow opens this port on a wide CIDR minus the metadata IPs). | `number` | `443` | no |
| <a name="input_fail_on_red"></a> [fail\_on\_red](#input\_fail\_on\_red) | Block apply when the preflight verdict is red. | `bool` | `true` | no |
| <a name="input_install_tier_override"></a> [install\_tier\_override](#input\_install\_tier\_override) | Force the install tier ("A"\|"B") instead of deriving it from the preflight report. Empty = derive. | `string` | `""` | no |
| <a name="input_kube_context"></a> [kube\_context](#input\_kube\_context) | Optional kubeconfig context name. Empty uses the current-context. | `string` | `""` | no |
| <a name="input_kubeconfig_path"></a> [kubeconfig\_path](#input\_kubeconfig\_path) | Path to the kubeconfig granting access to the EXISTING cluster. | `string` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Workload namespace. | `string` | `"workload-system"` | no |
| <a name="input_observability_enabled"></a> [observability\_enabled](#input\_observability\_enabled) | Apply the in-cluster ServiceMonitor + dashboard. Disable if the Prometheus-operator CRD is absent. | `bool` | `true` | no |
| <a name="input_operator_image_repository"></a> [operator\_image\_repository](#input\_operator\_image\_repository) | Operator image repository. | `string` | `"ghcr.io/ops-dev/workload-operator"` | no |
| <a name="input_operator_image_tag"></a> [operator\_image\_tag](#input\_operator\_image\_tag) | Operator image tag. | `string` | `"0.1.0"` | no |
| <a name="input_preflight_binary"></a> [preflight\_binary](#input\_preflight\_binary) | Absolute path to the preflight binary (operator/cmd/preflight build output). | `string` | n/a | yes |
| <a name="input_psa_enforce_level"></a> [psa\_enforce\_level](#input\_psa\_enforce\_level) | Pod Security Admission enforce level for the workload namespace: "restricted" (default, secure floor), "baseline" (permits root images — required if the workload runs as root), or "privileged". audit/warn always track restricted. | `string` | `"restricted"` | no |
| <a name="input_workload_name"></a> [workload\_name](#input\_workload\_name) | Workload name (CR metadata.name / chart .Values.name). | `string` | n/a | yes |
| <a name="input_workload_spec_yaml"></a> [workload\_spec\_yaml](#input\_workload\_spec\_yaml) | The Workload spec as YAML — the single source of the workload's shape, fed to both install<br/>tiers. Fields match the Workload CRD spec and charts/workload values.schema.json (minus<br/>name/namespace): image, port, autoscale{minReplicas,maxReplicas,targetCPUUtilization}, and<br/>optionally livenessProbe, readinessProbe, resources, securityContext, podSecurityContext,<br/>rolloutStrategy, ingressClass, ingress. Provide a file with `file("workload.yaml")` or inline<br/>heredoc. Per-cloud values are merged into this YAML at the call site. | `string` | n/a | yes |
| <a name="input_workload_wait_for_ready"></a> [workload\_wait\_for\_ready](#input\_workload\_wait\_for\_ready) | Tier A only: block the apply until the operator sets Ready=True. Disable on rate-limited/slow clusters; confirm readiness out-of-band instead. | `bool` | `true` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_install_tier"></a> [install\_tier](#output\_install\_tier) | The install tier this deploy used (A operator / B namespaced manifests). |
| <a name="output_namespace"></a> [namespace](#output\_namespace) | The workload namespace. |
| <a name="output_preflight_report"></a> [preflight\_report](#output\_preflight\_report) | The full decoded preflight staged report (for the SE to read / persist as an artifact). |
| <a name="output_preflight_verdict"></a> [preflight\_verdict](#output\_preflight\_verdict) | green \| amber \| red. |
| <a name="output_workload_name"></a> [workload\_name](#output\_workload\_name) | The deployed workload name. |
<!-- END_TF_DOCS -->
