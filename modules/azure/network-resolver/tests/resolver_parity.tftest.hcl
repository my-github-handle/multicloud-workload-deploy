# Asserts provision-mode and BYO-mode produce an IDENTICAL output interface
# ({vpc_id (string), subnet_ids (list), egress_path_ref (string)}) — the
# create-vs-lookup parity guarantee. command = plan; BYO data sources are mocked
# so no Azure subscription is needed.

mock_provider "azurerm" {
  mock_data "azurerm_virtual_network" {
    defaults = {
      id = "/subscriptions/0000/resourceGroups/byo-rg/providers/Microsoft.Network/virtualNetworks/byo-vnet"
    }
  }
  mock_data "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/0000/resourceGroups/byo-rg/providers/Microsoft.Network/virtualNetworks/byo-vnet/subnets/byo-nodes"
    }
  }
}

run "provision_mode_outputs" {
  command = plan

  variables {
    mode                        = "provision"
    provisioned_vnet_id         = "/subscriptions/0000/resourceGroups/prov-rg/providers/Microsoft.Network/virtualNetworks/prov-vnet"
    provisioned_subnet_ids      = ["/subscriptions/0000/resourceGroups/prov-rg/providers/Microsoft.Network/virtualNetworks/prov-vnet/subnets/nodes"]
    provisioned_egress_path_ref = "/subscriptions/0000/resourceGroups/prov-rg/providers/Microsoft.Network/azureFirewalls/prov-fw"
  }

  assert {
    condition     = output.vpc_id == "/subscriptions/0000/resourceGroups/prov-rg/providers/Microsoft.Network/virtualNetworks/prov-vnet"
    error_message = "provision mode must pass the provisioned VNet ID straight through."
  }
  assert {
    condition     = length(output.subnet_ids) == 1
    error_message = "provision mode must expose the provisioned subnet IDs."
  }
  assert {
    condition     = output.egress_path_ref == "/subscriptions/0000/resourceGroups/prov-rg/providers/Microsoft.Network/azureFirewalls/prov-fw"
    error_message = "provision mode must pass the provisioned egress path ref through."
  }
}

run "byo_mode_outputs" {
  command = plan

  variables {
    mode                    = "byo"
    byo_resource_group_name = "byo-rg"
    byo_vnet_name           = "byo-vnet"
    byo_subnet_names        = ["byo-nodes"]
    byo_egress_path_ref     = ""
  }

  # Same three output keys, same types, populated from the looked-up VNet/subnet.
  assert {
    condition     = output.vpc_id == "/subscriptions/0000/resourceGroups/byo-rg/providers/Microsoft.Network/virtualNetworks/byo-vnet"
    error_message = "BYO mode must expose the looked-up VNet ID under the same output key (vpc_id)."
  }
  assert {
    condition     = length(output.subnet_ids) == 1
    error_message = "BYO mode must expose the looked-up subnet IDs as a list, same shape as provision mode."
  }
  assert {
    condition     = output.egress_path_ref == ""
    error_message = "BYO mode egress_path_ref must still be a string (empty when the customer owns the edge firewall)."
  }
}
