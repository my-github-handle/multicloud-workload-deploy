locals {
  count_enabled = var.enabled ? 1 : 0

  # Minimal Grafana dashboard JSON for operator reconcile health. Kept inline (single panel) —
  # richer dashboards are added later without an interface change.
  dashboard_json = jsonencode({
    title         = "Workload Operator"
    uid           = "workload-operator"
    schemaVersion = 39
    panels = [
      {
        type  = "timeseries"
        title = "Reconcile rate"
        targets = [
          { expr = "sum(rate(controller_runtime_reconcile_total[5m])) by (controller)" }
        ]
        gridPos = { h = 8, w = 24, x = 0, y = 0 }
      }
    ]
  })

  # ServiceMonitor for the operator's controller-runtime metrics endpoint. ServiceMonitor is a
  # Prometheus-operator CRD, so we apply it via kubectl_manifest (raw YAML, no plan-time CRD schema
  # discovery) rather than kubernetes_manifest — the latter would require the Prometheus-operator
  # CRD reachable on the cluster even to plan, breaking offline plan/test and single-apply ordering.
  servicemonitor_manifest = {
    apiVersion = "monitoring.coreos.com/v1"
    kind       = "ServiceMonitor"
    metadata = {
      name      = "workload-operator"
      namespace = var.namespace
      labels    = var.operator_metrics_label_selector
    }
    spec = {
      selector = {
        matchLabels = var.operator_metrics_label_selector
      }
      namespaceSelector = {
        matchNames = [var.namespace]
      }
      endpoints = [
        {
          port     = var.metrics_port_name
          interval = var.scrape_interval
          path     = "/metrics"
        }
      ]
    }
  }
}

resource "kubectl_manifest" "operator_servicemonitor" {
  count = local.count_enabled

  yaml_body         = yamlencode(local.servicemonitor_manifest)
  server_side_apply = true
}

# Grafana dashboard ConfigMap. The Grafana sidecar discovers it via the dashboard label and loads
# the JSON.
resource "kubernetes_config_map" "grafana_dashboard" {
  count = local.count_enabled

  metadata {
    name      = "workload-operator-dashboard"
    namespace = var.namespace
    labels    = var.grafana_dashboard_label
  }
  data = {
    "workload-operator.json" = local.dashboard_json
  }
}
