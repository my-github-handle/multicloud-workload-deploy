# Ship the rendered custom role + bindings as reviewable artifacts so a customer
# can inspect / attach them.
locals {
  custom_role_doc = jsonencode({
    role_id = replace("${var.name}_runtime", "-", "_")
    title   = "${var.name} workload runtime (least-privilege)"
    permissions = [
      "cloudkms.cryptoKeyVersions.useToDecrypt",
      "cloudkms.cryptoKeyVersions.useToEncrypt",
      "secretmanager.versions.access",
      "artifactregistry.repositories.downloadArtifacts",
    ]
    note = "No primitive roles (owner/editor/viewer), no wildcards. Verbs pinned to resolved resources via resource-level bindings."
  })

  bindings_doc = jsonencode({
    kms_key_id = var.kms_key_id
    kms_role   = "(custom) ${var.name}_runtime — useToDecrypt/useToEncrypt on this key only"
    secrets = {
      scope  = "prefix"
      prefix = "projects/${var.project_id}/secrets/${var.secret_name_prefix}"
      role   = "(custom) ${var.name}_runtime — versions.access on secrets matching the prefix (IAM-condition scoped; no per-secret-id dependency)"
    }
    repos = [for r in var.artifact_registry_repo_ids : { repo = r, role = "roles/artifactregistry.reader on this repo only" }]
  })

  wi_binding_doc = jsonencode({
    google_service_account = var.mode == "provision" ? "${var.name}-workload@${var.project_id}.iam.gserviceaccount.com" : var.provided_gsa_email
    role                   = "roles/iam.workloadIdentityUser"
    member                 = local.wi_member
    ksa_annotation         = { "iam.gke.io/gcp-service-account" = var.mode == "provision" ? "${var.name}-workload@${var.project_id}.iam.gserviceaccount.com" : var.provided_gsa_email }
  })

  # --- DEPLOY-TIME policy artifact ---
  # Both the runtime AND the deploy-time identity policies are shipped as
  # reviewable, versioned artifacts — not the deploy-time set living ONLY as a Go
  # requiredDeployPermissions slice. This is the rendered deploy-time custom role:
  # the create/manage permissions the deploy identity needs for the gcp-full path,
  # scoped to this project, with NO primitive roles and NO wildcards. The Go
  # requiredDeployPermissions slice (operator/internal/cloud/gcp/identity.go) is
  # asserted CONSISTENT with this list in the golden test so the preflight probe
  # and the rendered artifact cannot drift.
  deploy_permissions = [
    "cloudkms.keyRings.create",
    "cloudkms.cryptoKeys.create",
    "secretmanager.secrets.create",
    "iam.roles.create",
    "iam.serviceAccounts.create",
    "container.clusters.create",
    "compute.networks.create",
  ]

  deploy_role_doc = jsonencode({
    role_id     = replace("${var.name}_deploy", "-", "_")
    title       = "${var.name} deploy-time (least-privilege, gcp-full path)"
    permissions = local.deploy_permissions
    note        = "Deploy-time create/manage permissions for the gcp-full path. No primitive roles (owner/editor/viewer), no wildcards. Asserted consistent with the Go requiredDeployPermissions probe."
  })
}

# Runtime workload-identity policy artifact (custom-role.json).
resource "local_file" "custom_role" {
  filename        = "${path.module}/${var.artifacts_dir}/runtime-policy/custom-role.json"
  content         = local.custom_role_doc
  file_permission = "0644"
}

# Deploy-time identity policy artifact (deploy-policy/role.json) — both the
# runtime and the deploy-time policy are shipped as reviewable files.
resource "local_file" "deploy_role" {
  filename        = "${path.module}/${var.artifacts_dir}/deploy-policy/role.json"
  content         = local.deploy_role_doc
  file_permission = "0644"
}

resource "local_file" "bindings" {
  filename        = "${path.module}/${var.artifacts_dir}/runtime-policy/bindings.json"
  content         = local.bindings_doc
  file_permission = "0644"
}

resource "local_file" "wi_binding" {
  filename        = "${path.module}/${var.artifacts_dir}/runtime-policy/wi-binding.json"
  content         = local.wi_binding_doc
  file_permission = "0644"
}
