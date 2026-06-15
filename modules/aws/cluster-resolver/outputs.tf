output "endpoint" {
  description = "Resolved cluster API endpoint — the FULL https:// URL (bare host normalized inside the resolver). Identical shape in both modes."
  value       = local.resolved_endpoint
}

output "ca" {
  description = "Resolved base64 cluster CA data."
  value       = local.resolved_ca
}

output "auth" {
  description = "Tagged auth object for the kubernetes/helm/kubectl providers. Shape: { kind = \"exec\"|\"token\", exec = { api_version, command, args }, token }. The AWS default is the EKS exec-plugin form (aws eks get-token), which avoids data.aws_eks_cluster_auth token churn. Consumers switch on auth.kind."
  value       = local.resolved_auth
  # The exec form carries no secret (the token field is null), so the object is
  # not marked sensitive — the provider block destructures it directly. When
  # kind == "token", the consumer marks its own provider token sensitive.
}
