terraform {
  required_version = ">= 1.7.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "4.77.0"
    }
    # SecretProviderClass is a Secrets-Store-CSI CRD applied as raw YAML with no
    # plan-time CRD schema discovery, so the module plans offline and applies once
    # the CSI add-on's CRD is present. kubernetes_manifest would require the CRD
    # reachable even to plan, breaking single-apply on a fresh cluster.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
}
