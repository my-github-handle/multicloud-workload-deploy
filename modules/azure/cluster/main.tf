# Disk encryption set bound to the resolved Key Vault key — encrypts node OS/data
# disks at rest with the customer-managed key.
resource "azurerm_disk_encryption_set" "this" {
  name                = "${var.name}-des"
  location            = var.location
  resource_group_name = var.resource_group_name
  key_vault_key_id    = var.key_id
  tags                = var.tags

  identity {
    type = "SystemAssigned"
  }
}

# The DES system identity needs to read/wrap the key. Scoped to the resolved
# vault only (no wildcards), consistent with the iam least-privilege model.
resource "azurerm_role_assignment" "des_key_access" {
  scope                = var.key_vault_id
  role_definition_name = "Key Vault Crypto Service Encryption User"
  principal_id         = azurerm_disk_encryption_set.this.identity[0].principal_id
}

# Hardened private AKS. Cilium is the dataplane, selected at cluster creation
# (network_data_plane = "cilium" with Azure CNI Overlay) — there is no separate
# Cilium helm_release; the dataplane selection here is the entire Cilium install.
resource "azurerm_kubernetes_cluster" "this" {
  name                   = var.name
  location               = var.location
  resource_group_name    = var.resource_group_name
  kubernetes_version     = var.k8s_version
  dns_prefix             = var.name
  disk_encryption_set_id = azurerm_disk_encryption_set.this.id

  # Private cluster by default; flip to a public endpoint with an IP allowlist
  # only for out-of-VNet testing (see variables).
  private_cluster_enabled = var.private_cluster_enabled

  # Public test mode only: restrict the public endpoint to the supplied CIDRs.
  # AKS rejects this block on a private cluster, so it is emitted only when public.
  dynamic "api_server_access_profile" {
    for_each = !var.private_cluster_enabled && length(var.api_server_authorized_ip_ranges) > 0 ? [1] : []
    content {
      authorized_ip_ranges = var.api_server_authorized_ip_ranges
    }
  }

  # Entra-only by default (no local Kubernetes accounts). When true the
  # kube_config cert/key are empty and the resolver emits exec (kubelogin) auth.
  local_account_disabled = var.local_account_disabled

  # Workload Identity + OIDC issuer (the iam federated credential depends on the
  # issuer URL this exposes).
  oidc_issuer_enabled       = true
  workload_identity_enabled = true

  # AKS-managed Entra RBAC for Kubernetes authorization (no static admin).
  role_based_access_control_enabled = true

  # Entra (AAD) integration. Required whenever local accounts are disabled
  # (AKS rejects local_account_disabled = true without it). Azure RBAC for
  # Kubernetes authorization; optional admin groups get cluster-admin.
  dynamic "azure_active_directory_role_based_access_control" {
    for_each = var.local_account_disabled ? [1] : []
    content {
      azure_rbac_enabled     = true
      admin_group_object_ids = var.admin_group_object_ids
    }
  }

  # Container Insights → the same Log Analytics workspace as the control-plane
  # diagnostic settings.
  oms_agent {
    log_analytics_workspace_id = var.log_analytics_workspace_id
  }

  # Azure Policy add-on (governance) + the Key Vault Secrets Provider CSI add-on
  # (the secrets module's SecretProviderClass mounts from it).
  azure_policy_enabled = true

  key_vault_secrets_provider {
    secret_rotation_enabled = true
  }

  default_node_pool {
    name                 = "system"
    vm_size              = var.node_vm_size
    vnet_subnet_id       = var.vnet_subnet_id
    auto_scaling_enabled = true
    min_count            = var.node_min_count
    max_count            = var.node_max_count
    max_pods             = var.max_pods
  }

  identity {
    type = "SystemAssigned"
  }

  # Azure CNI Overlay + Cilium dataplane. Pods draw from pod_cidr (overlay, not
  # VNet IPs); services from service_cidr (virtual ClusterIPs). Egress is forced
  # through the Azure Firewall by the network module's UDR, so outbound type is
  # userDefinedRouting.
  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    network_data_plane  = "cilium"
    network_policy      = "cilium"
    pod_cidr            = var.pod_cidr
    service_cidr        = var.service_cidr
    dns_service_ip      = var.dns_service_ip
    outbound_type       = "userDefinedRouting"
  }

  tags = var.tags
}

# Control-plane diagnostic settings → Log Analytics (kube-audit, apiserver, etc.).
resource "azurerm_monitor_diagnostic_setting" "cluster" {
  name                       = "${var.name}-diag"
  target_resource_id         = azurerm_kubernetes_cluster.this.id
  log_analytics_workspace_id = var.log_analytics_workspace_id

  enabled_log {
    category = "kube-audit"
  }
  enabled_log {
    category = "kube-apiserver"
  }
  enabled_log {
    category = "kube-controller-manager"
  }
  enabled_log {
    category = "guard"
  }

  enabled_metric {
    category = "AllMetrics"
  }
}
