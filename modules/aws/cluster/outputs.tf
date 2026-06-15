output "cluster_name" {
  description = "EKS cluster name."
  value       = aws_eks_cluster.this.name
}

output "endpoint" {
  description = "EKS API server endpoint (private)."
  value       = aws_eks_cluster.this.endpoint
}

output "ca" {
  description = "Base64-encoded cluster CA certificate."
  value       = aws_eks_cluster.this.certificate_authority[0].data
}

output "oidc_provider_arn" {
  description = "IAM OIDC provider ARN — consumed by the iam module's IRSA trust policy."
  value       = aws_iam_openid_connect_provider.this.arn
}

output "oidc_issuer_url" {
  description = "OIDC issuer URL — consumed by the iam module for the sub/aud conditions."
  value       = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

output "node_role_arn" {
  description = "Managed node group IAM role ARN."
  value       = aws_iam_role.node.arn
}

output "cluster_security_group_id" {
  description = "The EKS-managed cluster security group ID."
  value       = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id
}
