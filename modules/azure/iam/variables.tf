variable "mode" {
  description = "\"provision\" creates the UAMI, federated credential, custom role + assignments; \"byo\" emits the role-definition + federated-credential docs and resolves a customer-supplied UAMI."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "name" {
  description = "Name prefix for the managed identity and custom role."
  type        = string
}

variable "location" {
  description = "Azure region for the UAMI."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group the UAMI is created in."
  type        = string
}

variable "scope_id" {
  description = "Scope the custom role DEFINITION is assignable at (typically the resource group ID). Role ASSIGNMENTS are scoped narrower — to the resolved Key Vault / secret. No subscription-wide assignment."
  type        = string
}

variable "oidc_issuer_url" {
  description = "AKS OIDC issuer URL (from the cluster module) for the federated identity credential subject/issuer."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace of the workload + connect-agent ServiceAccount."
  type        = string
}

variable "service_account" {
  description = "Kubernetes ServiceAccount name bound to the UAMI via the federated credential (workload + connect-agent)."
  type        = string
  default     = "workload"
}

variable "key_vault_id" {
  description = "Resolved Key Vault ID (from kms module). Role assignment scopes Key Vault key decrypt/wrap-unwrap to THIS vault only."
  type        = string
}

variable "key_id" {
  description = "Resolved Key Vault Key ID (from kms module) — recorded in the reviewable artifact; the assignment is scoped at the vault."
  type        = string
}

# No secret_ids input: the Key Vault role assignment is scoped at the vault level
# (key_vault_id), so iam never depends on secrets — keeping the dependency
# one-directional (secrets consumes iam.uami_client_id, not the reverse).

variable "acr_id" {
  description = "Azure Container Registry resource ID the workload pulls images from. AcrPull is assigned scoped to THIS registry only. Empty to skip."
  type        = string
  default     = ""
}

variable "provided_uami_id" {
  description = "Existing user-assigned managed identity resource ID to resolve (byo-identity mode). The module still emits the role + federated-credential docs."
  type        = string
  default     = ""
}

variable "provided_uami_client_id" {
  description = "Client ID of the BYO UAMI (byo-identity mode)."
  type        = string
  default     = ""
}

variable "artifacts_dir" {
  description = "Absolute directory to write the reviewable role-definition, deploy-time policy, and federated-credential JSON artifacts into. Roots pass a dir rooted at path.root so artifacts stay out of the reusable module tree."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to created identity resources."
  type        = map(string)
  default     = {}
}
