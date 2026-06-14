terraform {
  required_version = ">= 1.7.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.13"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 2.3"
    }
    # Used by the workload (Tier A Workload CR) and k8s-observability (ServiceMonitor) modules:
    # applies CRD-typed manifests as raw YAML without plan-time CRD schema discovery, preserving
    # single-apply on a fresh cluster.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
  # Backend intentionally unset — the consumer wires layered remote state per their environment.
}
