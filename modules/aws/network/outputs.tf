output "vpc_id" {
  description = "Provisioned VPC ID."
  value       = aws_vpc.this.id
}

output "private_subnet_ids" {
  description = "Node subnet IDs (secondary CIDR, egress forced through the Network Firewall). Where the cluster module places nodes; named private_subnet_ids for cross-cloud resolver uniformity."
  value       = [for az in var.azs : aws_subnet.node[az].id]
}

output "pod_subnet_ids" {
  description = "Pod subnet IDs (secondary CIDR) from which Cilium ENI mode allocates pod IPs."
  value       = [for az in var.azs : aws_subnet.pod[az].id]
}

output "firewall_subnet_ids" {
  description = "Dedicated Network Firewall endpoint subnet IDs (primary CIDR, auto-routed to NAT)."
  value       = [for az in var.azs : aws_subnet.firewall[az].id]
}

output "public_subnet_ids" {
  description = "Public (NAT/LB) subnet IDs (primary CIDR)."
  value       = [for az in var.azs : aws_subnet.public[az].id]
}

output "egress_path_ref" {
  description = "Reference to the controlled egress path (the Network Firewall ARN). The resolver re-exports this uniformly."
  value       = aws_networkfirewall_firewall.egress.arn
}

output "flow_log_bucket_arn" {
  description = "ARN of the customer-owned, retention-locked S3 bucket holding VPC Flow Logs (the always-on audit floor)."
  value       = aws_s3_bucket.flow_logs.arn
}
