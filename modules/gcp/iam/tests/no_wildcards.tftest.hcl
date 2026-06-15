# Plan-time assertions that BOTH the runtime and deploy-time rendered roles are
# wildcard-free and use NO primitive roles. On GCP "wildcard-free" also means no
# primitive roles (roles/owner|editor|viewer) and no `*` in any custom-role
# permission. The deploy-time set is additionally asserted CONSISTENT with the Go
# requiredDeployPermissions probe so probe and artifact cannot drift. All three
# outputs are known at plan time, so command = plan needs no GCP project.

variables {
  name                       = "demo"
  project_id                 = "demo-project"
  mode                       = "provision"
  namespace                  = "workload-system"
  k8s_service_account        = "workload"
  kms_key_id                 = "projects/demo-project/locations/us-central1/keyRings/demo/cryptoKeys/demo"
  secret_name_prefix         = "demo-"
  artifact_registry_repo_ids = ["projects/demo-project/locations/us-central1/repositories/workload"]
}

run "runtime_role_is_wildcard_free_and_no_primitive_roles" {
  command = plan

  # No permission contains a wildcard.
  assert {
    condition     = length(regexall("\\*", output.custom_role_json)) == 0
    error_message = "runtime custom role must not contain any '*' wildcard permission."
  }
  # No primitive roles anywhere in the rendered role doc.
  assert {
    condition = (
      length(regexall("roles/owner", output.custom_role_json)) == 0 &&
      length(regexall("roles/editor", output.custom_role_json)) == 0 &&
      length(regexall("roles/viewer", output.custom_role_json)) == 0
    )
    error_message = "runtime custom role must not reference primitive roles (owner/editor/viewer)."
  }
  # All four enumerated least-privilege verbs are present (action-derived set); a
  # complete assertion so runtime-role drift cannot slip past this guard.
  assert {
    condition = (
      strcontains(output.custom_role_json, "cloudkms.cryptoKeyVersions.useToDecrypt") &&
      strcontains(output.custom_role_json, "cloudkms.cryptoKeyVersions.useToEncrypt") &&
      strcontains(output.custom_role_json, "secretmanager.versions.access") &&
      strcontains(output.custom_role_json, "artifactregistry.repositories.downloadArtifacts")
    )
    error_message = "runtime custom role must enumerate all four action-derived verbs (KMS encrypt/decrypt, Secret Manager access, Artifact Registry pull)."
  }
}

run "deploy_role_is_wildcard_free_and_no_primitive_roles" {
  command = plan

  # The deploy-time artifact is held to the SAME bar as the runtime one.
  assert {
    condition     = length(regexall("\\*", output.deploy_role_json)) == 0
    error_message = "deploy-time role must not contain any '*' wildcard permission."
  }
  assert {
    condition = (
      length(regexall("roles/owner", output.deploy_role_json)) == 0 &&
      length(regexall("roles/editor", output.deploy_role_json)) == 0 &&
      length(regexall("roles/viewer", output.deploy_role_json)) == 0
    )
    error_message = "deploy-time role must not reference primitive roles (owner/editor/viewer)."
  }
  # Anti-drift: every keystone permission in the Go requiredDeployPermissions
  # probe (operator/internal/cloud/gcp/identity.go) MUST appear in the rendered
  # deploy-time artifact. Keep this list in lockstep with the Go slice.
  assert {
    condition = (
      strcontains(output.deploy_role_json, "cloudkms.keyRings.create") &&
      strcontains(output.deploy_role_json, "cloudkms.cryptoKeys.create") &&
      strcontains(output.deploy_role_json, "secretmanager.secrets.create") &&
      strcontains(output.deploy_role_json, "iam.roles.create") &&
      strcontains(output.deploy_role_json, "iam.serviceAccounts.create") &&
      strcontains(output.deploy_role_json, "container.clusters.create") &&
      strcontains(output.deploy_role_json, "compute.networks.create")
    )
    error_message = "deploy-time role must contain every Go requiredDeployPermissions keystone permission (probe-vs-artifact consistency)."
  }
}
