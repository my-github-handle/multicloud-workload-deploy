# Terraform-native, co-located Azure data-source pre-checks. These complement the
# Go cloud.Provider staged checks: these run inside the plan graph and fail fast
# on resolved-resource sanity; the Go provider does the role-assignment / egress /
# key-state checks surfaced through the binary + report.

data "azurerm_client_config" "current" {}

data "azurerm_virtual_network" "resolved" {
  name                = element(split("/", var.vnet_id), length(split("/", var.vnet_id)) - 1)
  resource_group_name = element(split("/", var.vnet_id), index(split("/", var.vnet_id), "resourceGroups") + 1)
}

data "azurerm_key_vault" "resolved" {
  name                = element(split("/", var.key_vault_id), length(split("/", var.key_vault_id)) - 1)
  resource_group_name = element(split("/", var.key_vault_id), index(split("/", var.key_vault_id), "resourceGroups") + 1)
}

# Region match: the resolved VNet is in the intended region.
resource "terraform_data" "region_match" {
  input = var.location
  lifecycle {
    precondition {
      condition     = data.azurerm_virtual_network.resolved.location == var.location
      error_message = "Resolved VNet location ${data.azurerm_virtual_network.resolved.location} does not match the intended region ${var.location}."
    }
  }
}

# Key Vault purge protection (a disabled vault risks losing the envelope key).
resource "terraform_data" "kv_purge_protection" {
  input = var.key_vault_id
  lifecycle {
    precondition {
      condition     = data.azurerm_key_vault.resolved.purge_protection_enabled
      error_message = "Resolved Key Vault ${var.key_vault_id} does not have purge protection enabled."
    }
  }
}

# The resolved key id is non-empty (it belongs to the resolved vault).
resource "terraform_data" "key_present" {
  input = var.key_id
  lifecycle {
    precondition {
      condition     = var.key_id != "" && strcontains(var.key_id, data.azurerm_key_vault.resolved.vault_uri)
      error_message = "Resolved key id ${var.key_id} is empty or does not belong to the resolved Key Vault."
    }
  }
}
