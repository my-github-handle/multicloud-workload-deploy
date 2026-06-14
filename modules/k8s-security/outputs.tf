output "network_policy_names" {
  description = "Names of the NetworkPolicies applied to the namespace."
  value = [
    kubernetes_network_policy.default_deny.metadata[0].name,
    kubernetes_network_policy.allow.metadata[0].name,
  ]
}

output "namespace" {
  description = "The secured namespace."
  value       = var.namespace
}
