variable "namespace" {
  description = "Namespace where the operator runs and the ServiceMonitor/ConfigMap are created."
  type        = string
}

variable "operator_metrics_label_selector" {
  description = "Label selector matching the operator metrics Service/pods (controller-runtime metrics endpoint)."
  type        = map(string)
  default     = { "app.kubernetes.io/name" = "workload-operator" }
}

variable "metrics_port_name" {
  description = "Named port on the operator Service exposing Prometheus metrics."
  type        = string
  default     = "metrics"
}

variable "scrape_interval" {
  description = "Prometheus scrape interval for the operator metrics endpoint."
  type        = string
  default     = "30s"
}

variable "grafana_dashboard_label" {
  description = "Label key=value the Grafana sidecar watches for dashboard ConfigMaps."
  type        = map(string)
  default     = { "grafana_dashboard" = "1" }
}

variable "enabled" {
  description = "Toggle the whole observability layer. Disable when no Prometheus-operator CRDs are present (ServiceMonitor would not apply)."
  type        = bool
  default     = true
}
