output "servicemonitor_name" {
  description = "Name of the operator ServiceMonitor (empty when disabled)."
  value       = var.enabled ? local.servicemonitor_manifest.metadata.name : ""
}

output "dashboard_configmap_name" {
  description = "Name of the Grafana dashboard ConfigMap (empty when disabled)."
  value       = var.enabled ? kubernetes_config_map.grafana_dashboard[0].metadata[0].name : ""
}
