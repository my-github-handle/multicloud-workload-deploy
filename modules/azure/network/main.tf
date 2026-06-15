data "azurerm_client_config" "current" {}

# --- VNet + subnets ---------------------------------------------------------
resource "azurerm_virtual_network" "this" {
  name                = "${var.name}-vnet"
  location            = var.location
  resource_group_name = var.resource_group_name
  address_space       = var.address_space
  tags                = var.tags
}

# AKS node subnet — no public IPs; all egress is forced through the firewall
# by the route table below (private nodes + default-deny egress).
resource "azurerm_subnet" "nodes" {
  name                 = "${var.name}-nodes"
  resource_group_name  = var.resource_group_name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = [var.node_subnet_prefix]
}

# AzureFirewallSubnet — the name is mandated by Azure.
resource "azurerm_subnet" "firewall" {
  name                 = "AzureFirewallSubnet"
  resource_group_name  = var.resource_group_name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = [var.firewall_subnet_prefix]
}

# --- NAT Gateway for the node subnet (controlled, SNAT'd egress) ------------
resource "azurerm_public_ip" "nat" {
  name                = "${var.name}-nat-pip"
  location            = var.location
  resource_group_name = var.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = var.tags
}

resource "azurerm_nat_gateway" "this" {
  name                = "${var.name}-nat"
  location            = var.location
  resource_group_name = var.resource_group_name
  sku_name            = "Standard"
  tags                = var.tags
}

resource "azurerm_nat_gateway_public_ip_association" "this" {
  nat_gateway_id       = azurerm_nat_gateway.this.id
  public_ip_address_id = azurerm_public_ip.nat.id
}

resource "azurerm_subnet_nat_gateway_association" "nodes" {
  subnet_id      = azurerm_subnet.nodes.id
  nat_gateway_id = azurerm_nat_gateway.this.id
}

# --- Azure Firewall: FQDN + CIDR egress allowlist, default-deny -------------
resource "azurerm_public_ip" "firewall" {
  name                = "${var.name}-fw-pip"
  location            = var.location
  resource_group_name = var.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = var.tags
}

resource "azurerm_firewall_policy" "this" {
  name                = "${var.name}-fw-policy"
  location            = var.location
  resource_group_name = var.resource_group_name
  # threat_intel in Deny mode tightens the default-deny posture.
  threat_intelligence_mode = "Deny"
  tags                     = var.tags
}

# FQDN allowlist (default-deny everything else): the perimeter egress control,
# independent of the cluster CNI. application_rule_collection and
# network_rule_collection are sibling top-level blocks, never nested.
resource "azurerm_firewall_policy_rule_collection_group" "egress" {
  name               = "${var.name}-egress"
  firewall_policy_id = azurerm_firewall_policy.this.id
  priority           = 500

  application_rule_collection {
    name     = "allow-fqdns"
    priority = 500
    action   = "Allow"

    rule {
      name              = "allowed-fqdns"
      source_addresses  = var.address_space
      destination_fqdns = var.egress_allowed_fqdns
      protocols {
        type = "Https"
        port = 443
      }
      protocols {
        type = "Http"
        port = 80
      }
    }

    # AKS control-plane / bootstrap egress. With outbound_type = userDefinedRouting
    # the nodes reach the control plane and pull images THROUGH this firewall, so
    # the AzureKubernetesService FQDN tag (mcr, management.azure.com, *.azmk8s.io,
    # package repos, …) must be allowed or the cluster never finishes creating.
    dynamic "rule" {
      for_each = var.allow_aks_egress ? [1] : []
      content {
        name                  = "aks-required"
        source_addresses      = var.address_space
        destination_fqdn_tags = ["AzureKubernetesService"]
        protocols {
          type = "Https"
          port = 443
        }
        protocols {
          type = "Http"
          port = 80
        }
      }
    }
  }

  # AKS required NON-HTTP egress (NTP + the API-server tunnel). Network rule
  # collection — sibling of the application rule collection above.
  dynamic "network_rule_collection" {
    for_each = var.allow_aks_egress ? [1] : []
    content {
      name     = "aks-required-net"
      priority = 550
      action   = "Allow"
      rule {
        name                  = "ntp"
        source_addresses      = var.address_space
        destination_addresses = ["*"]
        destination_ports     = ["123"]
        protocols             = ["UDP"]
      }
      rule {
        name                  = "api-tunnel"
        source_addresses      = var.address_space
        destination_addresses = ["AzureCloud.${var.location}"]
        destination_ports     = ["1194", "9000", "443"]
        protocols             = ["UDP", "TCP"]
      }
    }
  }

  # Optional CIDR/service-tag egress (sibling of the application rule collection).
  dynamic "network_rule_collection" {
    for_each = length(var.egress_allowed_cidrs) > 0 ? [1] : []
    content {
      name     = "allow-cidrs"
      priority = 600
      action   = "Allow"
      rule {
        name                  = "allowed-cidrs"
        source_addresses      = var.address_space
        destination_addresses = var.egress_allowed_cidrs
        destination_ports     = ["443"]
        protocols             = ["TCP"]
      }
    }
  }
}

resource "azurerm_firewall" "this" {
  name                = "${var.name}-fw"
  location            = var.location
  resource_group_name = var.resource_group_name
  sku_name            = "AZFW_VNet"
  sku_tier            = "Standard"
  firewall_policy_id  = azurerm_firewall_policy.this.id

  ip_configuration {
    name                 = "fw-ipconfig"
    subnet_id            = azurerm_subnet.firewall.id
    public_ip_address_id = azurerm_public_ip.firewall.id
  }

  tags = var.tags
}

# --- UDR: force ALL node egress (0.0.0.0/0) through the firewall = default-deny
# Nothing leaves the node subnet except via the firewall, which only permits the
# allowlisted FQDNs/CIDRs.
resource "azurerm_route_table" "nodes" {
  name                = "${var.name}-nodes-rt"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_route" "default_to_firewall" {
  name                   = "default-to-firewall"
  resource_group_name    = var.resource_group_name
  route_table_name       = azurerm_route_table.nodes.name
  address_prefix         = "0.0.0.0/0"
  next_hop_type          = "VirtualAppliance"
  next_hop_in_ip_address = azurerm_firewall.this.ip_configuration[0].private_ip_address
}

resource "azurerm_subnet_route_table_association" "nodes" {
  subnet_id      = azurerm_subnet.nodes.id
  route_table_id = azurerm_route_table.nodes.id
}

# --- NSG on the node subnet (policy surface) --------------------------------
resource "azurerm_network_security_group" "nodes" {
  name                = "${var.name}-nodes-nsg"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_subnet_network_security_group_association" "nodes" {
  subnet_id                 = azurerm_subnet.nodes.id
  network_security_group_id = azurerm_network_security_group.nodes.id
}

# --- Customer-owned, IMMUTABLE Storage for VNet flow logs --------------------
# The always-on audit floor: CNI-independent, survives cluster compromise,
# immutable via a time-based retention policy with optional legal hold.
resource "azurerm_storage_account" "flow_logs" {
  # Globally unique, <= 24 lowercase-alphanumeric chars: name prefix + a hash over
  # (name + subscription + resource group) for stable per-deployment uniqueness.
  name                          = "${substr(lower(replace(var.name, "/[^a-z0-9]/", "")), 0, 11)}fl${substr(sha256("${var.name}-${data.azurerm_client_config.current.subscription_id}-${var.resource_group_name}"), 0, 11)}"
  location                      = var.location
  resource_group_name           = var.resource_group_name
  account_tier                  = "Standard"
  account_replication_type      = "GRS"
  min_tls_version               = "TLS1_2"
  public_network_access_enabled = false
  # Network Watcher flow logs write via the account access key, so shared key
  # access must stay enabled; hardened instead via private access + TLS1_2 + GRS +
  # container immutability.
  shared_access_key_enabled = true
  tags                      = var.tags
}

resource "azurerm_storage_container" "flow_logs" {
  name                  = "flow-logs"
  storage_account_id    = azurerm_storage_account.flow_logs.id
  container_access_type = "private"
}

# Time-based immutability (WORM): flow-log blobs cannot be modified or deleted
# before the retention period elapses — the retention-locked audit trail.
resource "azurerm_storage_container_immutability_policy" "flow_logs" {
  storage_container_resource_manager_id = azurerm_storage_container.flow_logs.id
  immutability_period_in_days           = var.flow_log_retention_days
  protected_append_writes_all_enabled   = true
  locked                                = false # set true out-of-band to make it irreversible
}

# Azure auto-provisions one NetworkWatcher_<region> per subscription/region in the
# NetworkWatcherRG resource group, and rejects a second watcher in the same region.
# Reference that managed watcher rather than creating our own.
data "azurerm_network_watcher" "this" {
  name                = var.network_watcher_name
  resource_group_name = var.network_watcher_resource_group
}

# VNet flow logs to the immutable blob store. The flow-log source is the VNet
# (target_resource_id); the node-subnet NSG is a policy surface, not the source.
resource "azurerm_network_watcher_flow_log" "nodes" {
  name                 = "${var.name}-vnet-flowlog"
  network_watcher_name = data.azurerm_network_watcher.this.name
  resource_group_name  = var.network_watcher_resource_group

  target_resource_id = azurerm_virtual_network.this.id
  storage_account_id = azurerm_storage_account.flow_logs.id
  enabled            = true
  version            = 2

  retention_policy {
    enabled = true
    days    = var.flow_log_retention_days
  }
}
