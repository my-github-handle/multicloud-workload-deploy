locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Provision: the Google service account the workload runs as.
resource "google_service_account" "workload" {
  count = local.is_provision ? 1 : 0

  account_id   = "${var.name}-workload"
  project      = var.project_id
  display_name = "${var.name} workload runtime SA"
}

# BYO-identity: resolve the customer-created GSA (they attach the emitted docs).
data "google_service_account" "byo" {
  count      = local.is_byo ? 1 : 0
  account_id = var.provided_gsa_email
  project    = var.project_id
}

locals {
  resolved_gsa_email  = local.is_provision ? google_service_account.workload[0].email : data.google_service_account.byo[0].email
  resolved_gsa_member = "serviceAccount:${local.resolved_gsa_email}"
}

# --- Workload Identity binding: bind the Kubernetes SA to the GSA via the
#     workloadIdentityUser role on the WI pool member. The member is
#     serviceAccount:PROJECT.svc.id.goog[NS/KSA]. ---
resource "google_service_account_iam_member" "wi_user" {
  count = local.is_provision ? 1 : 0

  service_account_id = google_service_account.workload[0].name
  role               = "roles/iam.workloadIdentityUser"
  member             = local.wi_member
}

# --- Resource-scoped bindings (the custom role's verbs pinned to THE resolved
#     resources only — no project-wide grant). ---

# cloudkms encrypt/decrypt on the resolved CryptoKey ONLY.
resource "google_kms_crypto_key_iam_member" "runtime_kms" {
  count = local.is_provision ? 1 : 0

  crypto_key_id = var.kms_key_id
  role          = google_project_iam_custom_role.runtime[0].id
  member        = local.resolved_gsa_member
}

# secretmanager.versions.access scoped to the workload's secrets by an IAM
# CONDITION on the secret-name prefix — NOT a per-secret-id binding. This breaks
# the iam↔secrets module cycle: iam needs only the deterministic prefix, never
# the secrets module's output ids. The condition restricts the grant to resources
# whose name starts with the project's secret path + prefix, so the grant is
# still scoped to exactly the workload's secrets.
resource "google_project_iam_member" "runtime_secrets" {
  count = local.is_provision && var.secret_name_prefix != "" ? 1 : 0

  project = var.project_id
  role    = google_project_iam_custom_role.runtime[0].id
  member  = local.resolved_gsa_member

  condition {
    title       = "${var.name}-secret-prefix"
    description = "Scope secretmanager.versions.access to secrets named with the workload prefix."
    expression  = "resource.name.startsWith(\"projects/${var.project_id}/secrets/${var.secret_name_prefix}\")"
  }
}

# artifactregistry.reader on the resolved repos ONLY (one binding each).
resource "google_artifact_registry_repository_iam_member" "runtime_repos" {
  for_each = local.is_provision ? toset(var.artifact_registry_repo_ids) : toset([])

  repository = each.value
  role       = "roles/artifactregistry.reader"
  member     = local.resolved_gsa_member
}
