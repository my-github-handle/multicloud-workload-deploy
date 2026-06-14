# Pure-Kubernetes deploy: the only credential is a kubeconfig / cluster access. There is
# deliberately NO aws/google/azurerm provider block — the BYOC fast path needs cluster access, not
# cloud-admin creds.
#
# This providers.tf is kubeconfig-driven and specific to _agnostic-deploy. The greenfield
# <cloud>-full roots cannot reuse it verbatim: they must author their own providers.tf fed from the
# cluster-resolver {endpoint, ca, auth} interface (the cluster does not exist until Phase 1 of
# their two-phase apply). Only the Layer 3 modules are reusable as-is (they declare no provider
# blocks).

provider "kubernetes" {
  config_path    = var.kubeconfig_path
  config_context = var.kube_context != "" ? var.kube_context : null
}

provider "helm" {
  kubernetes {
    config_path    = var.kubeconfig_path
    config_context = var.kube_context != "" ? var.kube_context : null
  }
}

# Same kubeconfig as the kubernetes/helm providers. alekc/kubectl applies the Tier A Workload CR +
# the ServiceMonitor without plan-time CRD schema discovery.
provider "kubectl" {
  config_path       = var.kubeconfig_path
  config_context    = var.kube_context != "" ? var.kube_context : null
  load_config_file  = true
  apply_retry_count = 3
}

# external provider needs no configuration; the preflight module supplies the program path and args.
provider "external" {}
