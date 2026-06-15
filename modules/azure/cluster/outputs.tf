output "cluster_name" {
  description = "AKS cluster name."
  value       = azurerm_kubernetes_cluster.this.name
}

output "host" {
  description = "AKS API server endpoint (private)."
  value       = azurerm_kubernetes_cluster.this.kube_config[0].host
  sensitive   = true
}

output "ca" {
  description = "Base64-encoded cluster CA certificate."
  value       = azurerm_kubernetes_cluster.this.kube_config[0].cluster_ca_certificate
  sensitive   = true
}

output "oidc_issuer_url" {
  description = "OIDC issuer URL — consumed by the iam module's federated identity credential."
  value       = azurerm_kubernetes_cluster.this.oidc_issuer_url
}

output "kube_config_raw" {
  description = "Raw kubeconfig for the kubernetes/helm providers + the preflight binary."
  value       = azurerm_kubernetes_cluster.this.kube_config_raw
  sensitive   = true
}

output "kube_config" {
  description = "Structured kube_config block (host/ca/client cert/key). When local_account_disabled = true (default) the client_certificate/client_key are EMPTY — only host + cluster_ca_certificate are usable, and provider auth must go through the resolver's exec (kubelogin) path."
  value       = azurerm_kubernetes_cluster.this.kube_config[0]
  sensitive   = true
}
