output "endpoint" {
  description = "Resolved cluster API endpoint (created or looked up) — identical shape both modes."
  value       = local.resolved_host
  sensitive   = true
}

output "ca" {
  description = "Resolved base64 cluster CA data."
  value       = local.resolved_ca
  sensitive   = true
}

output "auth" {
  description = "Tagged auth object for the kubernetes/helm/kubectl providers (the {endpoint, ca, auth} uniform interface). `auth.mode` is \"client_cert\" or \"exec\". client_cert carries client_certificate/client_key; exec carries `auth.exec` (kubelogin invocation) for Entra-only clusters whose kube_config cert/key are empty. The root's providers.tf branches on `auth.mode`."
  value = {
    mode               = local.resolved_auth.mode
    client_certificate = local.resolved_auth.client_certificate
    client_key         = local.resolved_auth.client_key
    exec               = local.resolved_auth.exec
  }
  sensitive = true
}
