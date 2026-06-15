output "endpoint" {
  description = "Resolved cluster API endpoint (created or looked up), https:// prefixed — identical shape both modes."
  value       = local.resolved_endpoint
}

output "ca" {
  description = "Resolved base64 cluster CA data."
  value       = local.resolved_ca
}

output "auth" {
  description = "Short-lived Google access token for the kubernetes/helm providers (the {endpoint, ca, auth} uniform interface). GKE token auth."
  value       = local.resolved_token
  sensitive   = true
}
