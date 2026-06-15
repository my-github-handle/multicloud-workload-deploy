# Assert the SecretProviderClass `objects` is a single well-formed YAML doc
# (array of per-object YAML scalars), not a double-encoded/escaped blob. The
# malformed double-encoded form only fails at pod-mount, so this plan-time shape
# assertion is the only offline gate that catches a regression.

mock_provider "azurerm" {}
mock_provider "kubectl" {}

variables {
  name                         = "demo"
  namespace                    = "workload-system"
  key_vault_id                 = "/subscriptions/x/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/demo-kv"
  key_vault_name               = "demo-kv"
  tenant_id                    = "11111111-1111-1111-1111-111111111111"
  uami_client_id               = "22222222-2222-2222-2222-222222222222"
  create_secret_provider_class = true
  secrets = {
    "db-password" = "x"
    "api-key"     = "y"
  }
}

run "objects_is_single_well_formed_yaml_doc" {
  command = plan

  # The wrapper is a real YAML map with a top-level `array` key (yamlencode quotes
  # keys), as a block-scalar list — not one quoted, escaped string.
  assert {
    condition     = strcontains(output.spc_objects_yaml, "\"array\":") && strcontains(output.spc_objects_yaml, "- |")
    error_message = "objects must be a YAML doc with a top-level array key holding block-scalar list elements."
  }
  # Each object renders as a per-object YAML scalar with its prefixed secret name.
  assert {
    condition     = strcontains(output.spc_objects_yaml, "\"objectName\": \"demo-db-password\"") && strcontains(output.spc_objects_yaml, "\"objectName\": \"demo-api-key\"")
    error_message = "each secret must render as a per-object YAML scalar with its prefixed name."
  }
  assert {
    condition     = strcontains(output.spc_objects_yaml, "\"objectType\": \"secret\"")
    error_message = "each object must declare objectType: secret."
  }
  # Regression guard: no doubly-escaped artifacts (escaped quotes or literal \n).
  assert {
    condition     = length(regexall("\\\\\"", output.spc_objects_yaml)) == 0 && length(regexall("\\\\n", output.spc_objects_yaml)) == 0
    error_message = "objects must not contain escaped quotes or literal \\n — that indicates the double-encode bug."
  }
}
