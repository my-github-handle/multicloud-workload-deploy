output "checks_passed" {
  description = "True once all co-located AWS data-source preconditions have evaluated (region/VPC/KMS)."
  value       = true
  depends_on = [
    terraform_data.region_match,
    terraform_data.vpc_available,
    terraform_data.kms_enabled,
  ]
}

output "account_id" {
  description = "The AWS account the deploy is running against (for the report/logs)."
  value       = data.aws_caller_identity.current.account_id
}
