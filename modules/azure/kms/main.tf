locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Provision: a customer-managed Key Vault with purge protection + soft delete,
# RBAC authorization, and a Key with an automatic rotation policy.
resource "azurerm_key_vault" "this" {
  count = local.is_provision ? 1 : 0

  name                       = "${var.name}-kv"
  location                   = var.location
  resource_group_name        = var.resource_group_name
  tenant_id                  = var.tenant_id
  sku_name                   = "standard"
  rbac_authorization_enabled = true
  purge_protection_enabled   = true
  soft_delete_retention_days = var.soft_delete_retention_days
  tags                       = var.tags

  # Default-deny vault network access; Azure trusted services (disk encryption
  # set, AKS CSI add-on) bypass. The deploy client needs data-plane access to
  # create the key — its IP is supplied at apply via allowed_ip_ranges (never
  # committed). Leave empty in VNet-connected/private contexts.
  network_acls {
    default_action = "Deny"
    bypass         = "AzureServices"
    ip_rules       = var.allowed_ip_ranges
  }
}

# Key expiry is managed by the rotation_policy below (expire_after); a static
# expiration_date would fight the automatic rotation.
# trivy:ignore:AVD-AZU-0014
resource "azurerm_key_vault_key" "this" {
  count = local.is_provision ? 1 : 0

  name         = var.key_name
  key_vault_id = azurerm_key_vault.this[0].id
  key_type     = "RSA"
  key_size     = 2048
  key_opts     = ["decrypt", "encrypt", "sign", "unwrapKey", "verify", "wrapKey"]

  # rotation_policy ISO-8601 durations must satisfy Azure's constraint: both
  # automatic.time_before_expiry and notify_before_expiry must be LESS than
  # expire_after. With the default rotation_months = 12 (expire_after ≈ 365d) the
  # P30D/P29D values are valid. A small rotation_months (e.g. 1 → P1M ≈ 30d) would
  # make P30D >= expire_after and FAIL apply; the rotation_months validation
  # forbids < 2 months to keep this valid.
  rotation_policy {
    automatic {
      time_before_expiry = "P30D"
    }
    expire_after         = "P${var.rotation_months}M"
    notify_before_expiry = "P29D"
  }
}

# BYO: resolve a supplied Key Vault + Key and verify they are usable.
data "azurerm_key_vault" "byo" {
  count               = local.is_byo ? 1 : 0
  name                = var.provided_key_vault_name
  resource_group_name = element(split("/", var.provided_key_vault_id), index(split("/", var.provided_key_vault_id), "resourceGroups") + 1)
}

data "azurerm_key_vault_key" "byo" {
  count        = local.is_byo ? 1 : 0
  name         = var.provided_key_name
  key_vault_id = data.azurerm_key_vault.byo[0].id
}

locals {
  resolved_key_vault_id  = local.is_provision ? azurerm_key_vault.this[0].id : data.azurerm_key_vault.byo[0].id
  resolved_key_vault_uri = local.is_provision ? azurerm_key_vault.this[0].vault_uri : data.azurerm_key_vault.byo[0].vault_uri
  resolved_key_id        = local.is_provision ? azurerm_key_vault_key.this[0].id : data.azurerm_key_vault_key.byo[0].id
  resolved_key_version   = local.is_provision ? azurerm_key_vault_key.this[0].version : data.azurerm_key_vault_key.byo[0].version

  # Surface BYO key-vault purge protection so callers/preflight can assert it.
  byo_purge_protection = local.is_byo ? data.azurerm_key_vault.byo[0].purge_protection_enabled : true
}

# Fail fast at plan time if a BYO Key Vault lacks purge protection (a disabled
# vault risks losing the envelope key under the workload's encrypted data).
resource "terraform_data" "key_usable" {
  input = local.resolved_key_id
  lifecycle {
    precondition {
      condition     = local.byo_purge_protection
      error_message = "The supplied BYO Key Vault does not have purge protection enabled; provide a vault with purge protection + soft delete."
    }
  }
}
