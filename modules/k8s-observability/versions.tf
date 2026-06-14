terraform {
  required_version = ">= 1.7.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
    # ServiceMonitor is a Prometheus-operator CRD. kubectl_manifest applies it as raw YAML with no
    # plan-time CRD schema discovery (unlike kubernetes_manifest), so the module plans offline and
    # applies in one run when the CRD is present.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
}
