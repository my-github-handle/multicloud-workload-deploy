# `k8s-observability` module

Applies the in-cluster observability layer for the operator.

- A **ServiceMonitor** (applied as raw YAML via `kubectl_manifest`, so it plans offline without the
  Prometheus-operator CRD present) selecting the operator's metrics Service and scraping its
  `controller-runtime` metrics endpoint.
- A **Grafana dashboard ConfigMap** for operator reconcile health, discovered by the Grafana
  sidecar via the dashboard label.

Toggle the whole layer with `enabled` (set `false` on clusters without the Prometheus-operator
CRDs). Scope is in-cluster only — cloud VPC flow logs live in the per-cloud network layer.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_kubectl"></a> [kubectl](#requirement\_kubectl) | ~> 2.0 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.30 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_kubectl"></a> [kubectl](#provider\_kubectl) | ~> 2.0 |
| <a name="provider_kubernetes"></a> [kubernetes](#provider\_kubernetes) | ~> 2.30 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [kubectl_manifest.operator_servicemonitor](https://registry.terraform.io/providers/alekc/kubectl/latest/docs/resources/manifest) | resource |
| [kubernetes_config_map.grafana_dashboard](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/config_map) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_enabled"></a> [enabled](#input\_enabled) | Toggle the whole observability layer. Disable when no Prometheus-operator CRDs are present (ServiceMonitor would not apply). | `bool` | `true` | no |
| <a name="input_grafana_dashboard_label"></a> [grafana\_dashboard\_label](#input\_grafana\_dashboard\_label) | Label key=value the Grafana sidecar watches for dashboard ConfigMaps. | `map(string)` | <pre>{<br/>  "grafana_dashboard": "1"<br/>}</pre> | no |
| <a name="input_metrics_port_name"></a> [metrics\_port\_name](#input\_metrics\_port\_name) | Named port on the operator Service exposing Prometheus metrics. | `string` | `"metrics"` | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Namespace where the operator runs and the ServiceMonitor/ConfigMap are created. | `string` | n/a | yes |
| <a name="input_operator_metrics_label_selector"></a> [operator\_metrics\_label\_selector](#input\_operator\_metrics\_label\_selector) | Label selector matching the operator metrics Service/pods (controller-runtime metrics endpoint). | `map(string)` | <pre>{<br/>  "app.kubernetes.io/name": "workload-operator"<br/>}</pre> | no |
| <a name="input_scrape_interval"></a> [scrape\_interval](#input\_scrape\_interval) | Prometheus scrape interval for the operator metrics endpoint. | `string` | `"30s"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_dashboard_configmap_name"></a> [dashboard\_configmap\_name](#output\_dashboard\_configmap\_name) | Name of the Grafana dashboard ConfigMap (empty when disabled). |
| <a name="output_servicemonitor_name"></a> [servicemonitor\_name](#output\_servicemonitor\_name) | Name of the operator ServiceMonitor (empty when disabled). |
<!-- END_TF_DOCS -->
