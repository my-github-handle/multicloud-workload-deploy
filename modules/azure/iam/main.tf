# Provision: the user-assigned managed identity the workload pods run as.
resource "azurerm_user_assigned_identity" "workload" {
  count = local.is_provision ? 1 : 0

  name                = "${var.name}-workload"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

# Federated identity credential: binds the Kubernetes ServiceAccount
# (system:serviceaccount:<ns>:<sa>) to the UAMI via the AKS OIDC issuer — this is
# Microsoft Entra Workload ID. No client secret; the pod's projected SA token is
# exchanged for an Entra token (workload identity, no static keys).
resource "azurerm_federated_identity_credential" "workload" {
  count = local.is_provision ? 1 : 0

  name                = "${var.name}-fic"
  resource_group_name = var.resource_group_name
  parent_id           = azurerm_user_assigned_identity.workload[0].id
  audience            = ["api://AzureADTokenExchange"]
  issuer              = var.oidc_issuer_url
  subject             = "system:serviceaccount:${var.namespace}:${var.service_account}"
}

# BYO-identity: resolve the customer-created UAMI (they attach the emitted role +
# federated-credential docs). The federated credential subject/issuer the customer
# must configure is emitted as a reviewable artifact (artifacts.tf).
data "azurerm_user_assigned_identity" "byo" {
  count               = local.is_byo ? 1 : 0
  name                = element(split("/", var.provided_uami_id), length(split("/", var.provided_uami_id)) - 1)
  resource_group_name = element(split("/", var.provided_uami_id), index(split("/", var.provided_uami_id), "resourceGroups") + 1)
}

locals {
  resolved_uami_client_id    = local.is_provision ? azurerm_user_assigned_identity.workload[0].client_id : var.provided_uami_client_id
  resolved_uami_principal_id = local.is_provision ? azurerm_user_assigned_identity.workload[0].principal_id : data.azurerm_user_assigned_identity.byo[0].principal_id
}
