locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# --- Custom role definition: action-derived, resource-scoped, NO wildcards and
#     NO built-in privileged roles. Lists ONLY the explicit control-plane actions
#     and Key Vault data-plane DataActions the workload runtime needs
#     (least-privilege model). ---
resource "azurerm_role_definition" "workload" {
  count = local.is_provision ? 1 : 0

  name        = "${var.name}-workload-runtime"
  scope       = var.scope_id
  description = "Least-privilege runtime role for the workload: Key Vault key decrypt/wrap-unwrap + secret get on the resolved vault, plus image pull. No wildcards, no Owner/Contributor."

  permissions {
    # Control-plane: read-only on the specific Key Vault (no management actions).
    actions = [
      "Microsoft.KeyVault/vaults/read",
    ]
    not_actions = []

    # Data-plane: the crypto operations against the envelope key + secret reads.
    # Equivalent to the built-in "Key Vault Crypto User" + "Key Vault Secrets
    # User" data roles, enumerated explicitly so the role cannot silently widen.
    data_actions = [
      "Microsoft.KeyVault/vaults/keys/read",
      "Microsoft.KeyVault/vaults/keys/decrypt/action",
      "Microsoft.KeyVault/vaults/keys/encrypt/action",
      "Microsoft.KeyVault/vaults/keys/wrap/action",
      "Microsoft.KeyVault/vaults/keys/unwrap/action",
      "Microsoft.KeyVault/vaults/secrets/getSecret/action",
      "Microsoft.KeyVault/vaults/secrets/readMetadata/action",
    ]
    not_data_actions = []
  }

  # The role is only ASSIGNABLE within the resolved scope — never subscription-wide.
  assignable_scopes = [var.scope_id]
}

# --- Role assignment: bind the custom role to the UAMI, scoped to the RESOLVED
#     Key Vault ONLY (not the resource group, not the subscription). ---
resource "azurerm_role_assignment" "keyvault" {
  count = local.is_provision ? 1 : 0

  scope              = var.key_vault_id
  role_definition_id = azurerm_role_definition.workload[0].role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.workload[0].principal_id
}

# --- AcrPull: scoped to the named registry only (read-only image pull). ---
resource "azurerm_role_assignment" "acr_pull" {
  count = local.is_provision && var.acr_id != "" ? 1 : 0

  scope                = var.acr_id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_user_assigned_identity.workload[0].principal_id
}
