locals {
  is_byo  = var.mode == "byo"
  is_exec = var.auth_mode == "exec"
}

# BYO: look up the existing cluster's kube_config (host + CA). For Entra-only /
# local-account-disabled clusters the kube_config cert/key are empty, so exec
# mode does not rely on them; client_cert mode uses them. Both modes feed the
# same tagged-auth model — no divergence between the resolver paths.
data "azurerm_kubernetes_cluster" "byo" {
  count               = local.is_byo ? 1 : 0
  name                = var.provided_cluster_name
  resource_group_name = var.resource_group_name
}

locals {
  resolved_host = local.is_byo ? data.azurerm_kubernetes_cluster.byo[0].kube_config[0].host : var.provisioned_host
  resolved_ca   = local.is_byo ? data.azurerm_kubernetes_cluster.byo[0].kube_config[0].cluster_ca_certificate : var.provisioned_ca

  resolved_cluster_name = local.is_byo ? var.provided_cluster_name : var.cluster_name_for_exec

  # client_cert fields — only populated/used in client_cert mode (empty otherwise).
  resolved_client_cert = local.is_exec ? "" : (local.is_byo ? data.azurerm_kubernetes_cluster.byo[0].kube_config[0].client_certificate : var.provisioned_client_certificate)
  resolved_client_key  = local.is_exec ? "" : (local.is_byo ? data.azurerm_kubernetes_cluster.byo[0].kube_config[0].client_key : var.provisioned_client_key)

  # Tagged auth object. `mode` tells the root which form to wire. Both forms
  # always carry every key (empty when unused) so the object type is uniform and
  # the parity test can assert a stable shape across modes.
  resolved_auth = {
    mode               = var.auth_mode
    client_certificate = local.resolved_client_cert
    client_key         = local.resolved_client_key
    # exec form: the kubelogin invocation the kubernetes/helm/kubectl providers run
    # to obtain an Entra token.
    exec = {
      command        = "kubelogin"
      args           = ["get-token", "--login", "azurecli", "--server-id", "6dae42f8-4368-4678-94ff-3960e28e3630"]
      resource_group = var.resource_group_name
      cluster_name   = local.resolved_cluster_name
    }
  }
}

# Co-located precondition: the resolved endpoint is present and, in exec mode, the
# cluster name kubelogin needs is set — fails the plan fast next to the module
# that produces the auth interface.
resource "terraform_data" "resolver_preflight" {
  input = local.resolved_host
  lifecycle {
    precondition {
      condition     = local.resolved_host != ""
      error_message = "cluster-resolver could not resolve a cluster endpoint (provision passthrough empty or BYO lookup failed)."
    }
    precondition {
      condition     = !local.is_exec || local.resolved_cluster_name != ""
      error_message = "exec auth_mode requires a cluster name for kubelogin (set cluster_name_for_exec in provision mode or provided_cluster_name in BYO mode)."
    }
  }
}
