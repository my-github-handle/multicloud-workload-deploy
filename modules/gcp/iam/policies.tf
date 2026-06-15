# --- Least-privilege CUSTOM ROLE ---
# A google_project_iam_custom_role listing ONLY the exact permissions the
# workload + connect-agent runtime needs. NO primitive roles (roles/owner,
# roles/editor, roles/viewer) and NO wildcards. Each permission is enumerated:
#   - cloudkms.cryptoKeyVersions.useToDecrypt / useToEncrypt  (envelope decrypt/encrypt with the CMK)
#   - secretmanager.versions.access                            (read the resolved secret material)
#   - artifactregistry.repositories.downloadArtifacts          (pull the workload image)
# Resource SCOPING (to THIS key / THESE secrets / THESE repos only) is enforced
# by the IAM bindings in main.tf — the custom role grants the verbs, the
# resource-level bindings pin them to the resolved resources.
resource "google_project_iam_custom_role" "runtime" {
  count = var.mode == "provision" ? 1 : 0

  project     = var.project_id
  role_id     = replace("${var.name}_runtime", "-", "_")
  title       = "${var.name} workload runtime (least-privilege)"
  description = "Action-derived least-privilege role for the workload + connect-agent runtime identity. No primitive roles, no wildcards."
  permissions = [
    "cloudkms.cryptoKeyVersions.useToDecrypt",
    "cloudkms.cryptoKeyVersions.useToEncrypt",
    "secretmanager.versions.access",
    "artifactregistry.repositories.downloadArtifacts",
  ]
}

locals {
  # Workload Identity pool member for this project. The GKE WI pool is
  # PROJECT.svc.id.goog; the member is the KSA in NS bound through it.
  wi_member = "serviceAccount:${var.project_id}.svc.id.goog[${var.namespace}/${var.k8s_service_account}]"
}
