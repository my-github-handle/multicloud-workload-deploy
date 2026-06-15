variable "mode" {
  description = "\"provision\" creates the GSA, custom role, bindings, and Workload Identity binding; \"byo\" emits the rendered role + WI-binding docs and resolves a customer-supplied GSA email."
  type        = string
  validation {
    condition     = contains(["provision", "byo"], var.mode)
    error_message = "mode must be \"provision\" or \"byo\"."
  }
}

variable "name" {
  description = "Name prefix for the service account and custom role."
  type        = string
}

variable "project_id" {
  description = "GCP project ID — bindings are scoped to resolved resources within this project (no project-wide primitive roles)."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace of the workload + connect-agent ServiceAccount."
  type        = string
}

variable "k8s_service_account" {
  description = "Kubernetes ServiceAccount name bound to the GSA via Workload Identity (workload + connect-agent)."
  type        = string
  default     = "workload"
}

variable "kms_key_id" {
  description = "Resolved CryptoKey id (from kms module). Runtime binding scopes cloudkms encrypt/decrypt to THIS key only — no project-wide grant."
  type        = string
}

# NOTE: iam does NOT take secrets.secret_ids as an input — that would form an
# iam↔secrets module cycle in the greenfield root (secrets needs the
# workload-identity member, iam would need the secret ids). Instead the runtime
# Secret Manager binding is scoped by a deterministic secret-name PREFIX
# (computed from name) so iam has NO hard dependency on the secrets module. On
# GCP a project-level binding of secretmanager.versions.access with an IAM
# condition on the secret name prefix scopes access to exactly the workload's
# secrets without enumerating their ids.
variable "secret_name_prefix" {
  description = "Deterministic Secret Manager secret-name prefix the workload's secrets share (e.g. \"<name>-\"). The runtime versions.access binding is scoped to secrets matching this prefix via an IAM condition — NO dependency on the secrets module (breaks the iam↔secrets cycle)."
  type        = string
  default     = ""
}

variable "artifact_registry_repo_ids" {
  description = "Artifact Registry repository ids the workload pulls from. Runtime binding scopes artifactregistry.reader to these repos only."
  type        = list(string)
  default     = []
}

variable "provided_gsa_email" {
  description = "Existing Google service account email to resolve (byo-identity mode). The module still emits the role + WI-binding docs for the customer to attach."
  type        = string
  default     = ""
}

variable "artifacts_dir" {
  description = "Directory to write the reviewable role/binding JSON artifacts into."
  type        = string
  default     = "artifacts"
}
