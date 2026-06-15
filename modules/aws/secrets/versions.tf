terraform {
  required_version = ">= 1.7.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    # SecretProviderClass is a Secrets-Store-CSI CRD. kubectl_manifest applies it as
    # raw YAML with no plan-time CRD schema discovery, so the module plans offline
    # and applies in the same run once the CSI driver's CRD is present.
    # kubernetes_manifest would require that CRD reachable on the cluster even to
    # PLAN, which breaks single-apply on a fresh greenfield cluster.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
}
