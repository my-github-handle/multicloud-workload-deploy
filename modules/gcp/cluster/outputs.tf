output "cluster_name" {
  description = "GKE cluster name."
  value       = module.gke.name
}

output "endpoint" {
  description = "GKE API server endpoint (private by default; public when enable_private_endpoint = false)."
  value       = module.gke.endpoint
}

output "ca" {
  description = "Base64-encoded cluster CA certificate."
  value       = module.gke.ca_certificate
}

output "location" {
  description = "Cluster location (region) — consumed by the cluster-resolver."
  value       = var.region
}

output "workload_identity_pool" {
  description = "The Workload Identity pool (PROJECT.svc.id.goog) the iam module binds the KSA through."
  value       = "${var.project_id}.svc.id.goog"
}
