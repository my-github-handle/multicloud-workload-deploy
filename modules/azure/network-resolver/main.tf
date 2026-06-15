locals {
  is_byo = var.mode == "byo"
}

# BYO lookups only run in byo mode (count gates them off in provision mode).
# This is the SINGLE create-vs-lookup branch in the whole Azure network path —
# the single branch lives only in the resolver; everything downstream receives
# identical inputs.
data "azurerm_virtual_network" "byo" {
  count               = local.is_byo ? 1 : 0
  name                = var.byo_vnet_name
  resource_group_name = var.byo_resource_group_name
}

data "azurerm_subnet" "byo" {
  count                = local.is_byo ? length(var.byo_subnet_names) : 0
  name                 = var.byo_subnet_names[count.index]
  virtual_network_name = var.byo_vnet_name
  resource_group_name  = var.byo_resource_group_name
}

locals {
  # Coalesce both modes into one uniform interface.
  resolved_vpc_id = local.is_byo ? data.azurerm_virtual_network.byo[0].id : var.provisioned_vnet_id

  resolved_subnet_ids = local.is_byo ? [for s in data.azurerm_subnet.byo : s.id] : var.provisioned_subnet_ids

  resolved_egress_path_ref = local.is_byo ? var.byo_egress_path_ref : var.provisioned_egress_path_ref
}
