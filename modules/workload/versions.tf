terraform {
  required_version = ">= 1.7.0"
  required_providers {
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.13"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
    # alekc/kubectl applies raw-YAML manifests WITHOUT plan-time CRD schema discovery (unlike
    # hashicorp/kubernetes' kubernetes_manifest), so the Tier A Workload CR can be planned before
    # its CRD exists and applied in the same run after k8s-platform installs it. This is what
    # preserves single-apply.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
}
