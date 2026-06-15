locals {
  # The custom role definition as a reviewable JSON document.
  role_definition_doc = {
    Name        = "${var.name}-workload-runtime"
    IsCustom    = true
    Description = "Least-privilege runtime role: Key Vault key decrypt/wrap-unwrap + secret get on the resolved vault, plus image pull. No wildcards, no Owner/Contributor."
    Actions     = ["Microsoft.KeyVault/vaults/read"]
    NotActions  = []
    DataActions = [
      "Microsoft.KeyVault/vaults/keys/read",
      "Microsoft.KeyVault/vaults/keys/decrypt/action",
      "Microsoft.KeyVault/vaults/keys/encrypt/action",
      "Microsoft.KeyVault/vaults/keys/wrap/action",
      "Microsoft.KeyVault/vaults/keys/unwrap/action",
      "Microsoft.KeyVault/vaults/secrets/getSecret/action",
      "Microsoft.KeyVault/vaults/secrets/readMetadata/action",
    ]
    NotDataActions   = []
    AssignableScopes = [var.scope_id]
    # Where the role is assigned (narrower than assignable): the vault + the ACR.
    AssignedScopes     = compact([var.key_vault_id, var.acr_id])
    KeyScoped          = var.key_id
    SecretsScopedVault = var.key_vault_id
  }

  # The deploy-time identity policy: the create/manage permissions the deploy
  # path needs, scoped to the deploy scope. Wildcard-free, same discipline as the
  # runtime role.
  deploy_policy_doc = {
    Name        = "${var.name}-deploy-time"
    IsCustom    = true
    Description = "Deploy-time identity policy: create/manage the workload's Key Vault, UAMI, federated credential, role definition + assignments, and cluster bindings — scoped to the deploy scope. No wildcards, no Owner/Contributor."
    Actions = [
      "Microsoft.ManagedIdentity/userAssignedIdentities/read",
      "Microsoft.ManagedIdentity/userAssignedIdentities/write",
      "Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials/read",
      "Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials/write",
      "Microsoft.Authorization/roleDefinitions/read",
      "Microsoft.Authorization/roleDefinitions/write",
      "Microsoft.Authorization/roleAssignments/read",
      "Microsoft.Authorization/roleAssignments/write",
      "Microsoft.KeyVault/vaults/read",
    ]
    NotActions       = []
    DataActions      = []
    NotDataActions   = []
    AssignableScopes = [var.scope_id]
    AssignedScopes   = [var.scope_id]
  }

  # The SA→UAMI binding contract (created in provision mode; configured by the
  # customer in BYO mode).
  federated_credential_doc = {
    issuer   = var.oidc_issuer_url
    subject  = "system:serviceaccount:${var.namespace}:${var.service_account}"
    audience = ["api://AzureADTokenExchange"]
  }
}

# Artifacts are written to the passed-in absolute dir, kept out of path.module so
# generated files never land inside the reusable module tree.
resource "local_file" "role_definition" {
  filename        = "${var.artifacts_dir}/role-definition.json"
  content         = jsonencode(local.role_definition_doc)
  file_permission = "0644"
}

# The deploy-time policy is rendered as a first-class reviewable artifact.
resource "local_file" "deploy_policy" {
  filename        = "${var.artifacts_dir}/deploy-policy.json"
  content         = jsonencode(local.deploy_policy_doc)
  file_permission = "0644"
}

resource "local_file" "federated_credential" {
  filename        = "${var.artifacts_dir}/federated-credential.json"
  content         = jsonencode(local.federated_credential_doc)
  file_permission = "0644"
}
