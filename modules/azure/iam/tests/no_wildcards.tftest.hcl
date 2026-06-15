# Plan-time assertions that the rendered custom role is wildcard-free and uses NO
# privileged built-in roles. Offline (command = plan).

mock_provider "azurerm" {}
mock_provider "local" {}

variables {
  name                = "demo"
  mode                = "provision"
  location            = "eastus"
  resource_group_name = "demo-rg"
  scope_id            = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/demo-rg"
  oidc_issuer_url     = "https://eastus.oic.prod-aks.azure.com/00000000/EXAMPLE/"
  namespace           = "workload-system"
  service_account     = "workload"
  key_vault_id        = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/demo-rg/providers/Microsoft.KeyVault/vaults/demo-kv"
  key_id              = "https://demo-kv.vault.azure.net/keys/demo/abcd"
  # NO secret_ids input — iam scopes secret access at the vault level.
  acr_id        = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/demo-rg/providers/Microsoft.ContainerRegistry/registries/demoacr"
  artifacts_dir = "/tmp/iam-test-artifacts" # absolute; avoids polluting the module tree
}

run "custom_role_is_wildcard_free_and_no_privileged_builtins" {
  command = plan

  # No wildcard in any Action / DataActions.
  assert {
    condition     = length(regexall("\\*", output.role_definition_json)) == 0
    error_message = "custom role must not contain any '*' wildcard (no Microsoft.KeyVault/*, no Microsoft.Authorization/*)."
  }
  # No privileged built-in role names embedded in the custom role doc.
  assert {
    condition = (
      length(regexall("\"Owner\"", output.role_definition_json)) == 0 &&
      length(regexall("\"Contributor\"", output.role_definition_json)) == 0 &&
      length(regexall("User Access Administrator", output.role_definition_json)) == 0
    )
    error_message = "custom role must not reference Owner/Contributor/User Access Administrator."
  }
  # The exact enumerated least-privilege DataActions are present (action-derived).
  assert {
    condition     = strcontains(output.role_definition_json, "Microsoft.KeyVault/vaults/keys/decrypt/action") && strcontains(output.role_definition_json, "Microsoft.KeyVault/vaults/secrets/getSecret/action")
    error_message = "custom role must enumerate the action-derived Key Vault crypto + secret DataActions."
  }
  # Assignable only at the supplied scope — never subscription-wide.
  assert {
    condition     = strcontains(output.role_definition_json, var.scope_id)
    error_message = "custom role AssignableScopes must be the resolved scope, not subscription-wide."
  }
}

# The SAME discipline on the DEPLOY-TIME policy artifact.
run "deploy_policy_is_wildcard_free_and_resource_pinned" {
  command = plan

  # No wildcard anywhere in the deploy-time policy.
  assert {
    condition     = length(regexall("\\*", output.deploy_policy_json)) == 0
    error_message = "deploy-time policy must not contain any '*' wildcard."
  }
  # No privileged built-in role names embedded in the deploy-time policy.
  assert {
    condition = (
      length(regexall("\"Owner\"", output.deploy_policy_json)) == 0 &&
      length(regexall("\"Contributor\"", output.deploy_policy_json)) == 0 &&
      length(regexall("User Access Administrator", output.deploy_policy_json)) == 0
    )
    error_message = "deploy-time policy must not reference Owner/Contributor/User Access Administrator."
  }
  # Resource-pinned to the deploy scope, never subscription-wide.
  assert {
    condition     = strcontains(output.deploy_policy_json, var.scope_id)
    error_message = "deploy-time policy must be pinned to the resolved scope, not subscription-wide."
  }
}
