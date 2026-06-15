variable "mode" {
  description = "\"provision\" creates the IRSA role and attaches the rendered policies; \"byo\" emits the policy+trust docs and resolves a customer-supplied role ARN."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "name" {
  description = "Name prefix for the IAM role and policies."
  type        = string
}

variable "region" {
  description = "AWS region — pinned into policy conditions (no account-wide grants)."
  type        = string
}

variable "account_id" {
  description = "AWS account ID — pinned into policy conditions."
  type        = string
}

variable "oidc_provider_arn" {
  description = "ARN of the EKS cluster's IAM OIDC provider (from the cluster module), for the IRSA trust policy."
  type        = string
}

variable "oidc_issuer_url" {
  description = "EKS OIDC issuer URL (https://oidc.eks...). The scheme is stripped internally to form the sub/aud condition keys."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace of the workload + connect-agent ServiceAccount."
  type        = string
}

variable "service_account" {
  description = "Kubernetes ServiceAccount name bound to this role (workload + connect-agent)."
  type        = string
  default     = "workload"
}

variable "kms_key_arn" {
  description = "Resolved CMK ARN (from the kms module). The runtime policy scopes kms:Decrypt/GenerateDataKey to THIS ARN only — no wildcards."
  type        = string
}

# The runtime policy scopes Secrets Manager access at the path-PREFIX level rather
# than per-secret-arn, so this module never consumes secrets.secret_arns. That keeps
# iam and secrets as independent siblings (no module dependency cycle): the grant
# `arn:...:secret:<prefix>-*` covers every secret the secrets module creates under
# the prefix.
variable "secret_path_prefix" {
  description = "Secrets Manager name prefix the workload's secrets live under (matches the name prefix passed to modules/aws/secrets). The runtime policy scopes secretsmanager:GetSecretValue to arn:...:secret:<prefix>-* — path-prefix scoped, not per-secret-arn."
  type        = string
}

variable "recorded_secret_arns" {
  description = "Optional concrete secret ARNs recorded in a companion artifact for reviewer visibility only. The live policy is scoped by secret_path_prefix, not these ARNs; may be left empty."
  type        = list(string)
  default     = []
}

variable "ecr_repo_arns" {
  description = "ECR repository ARNs the workload pulls from. The runtime policy scopes ECR pull actions to these repos only."
  type        = list(string)
  default     = []
}

variable "provided_role_arn" {
  description = "Existing IRSA role ARN to resolve (byo-identity mode). The module still emits the policy+trust docs for the customer to attach."
  type        = string
  default     = ""
}

variable "artifacts_dir" {
  description = "Directory (relative to the module) to write the reviewable policy JSON artifacts into."
  type        = string
  default     = "artifacts"
}

variable "tags" {
  description = "Tags applied to created IAM resources."
  type        = map(string)
  default     = {}
}
