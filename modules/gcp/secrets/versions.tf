terraform {
  required_version = ">= 1.7.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    # SecretProviderClass is a Secrets-Store-CSI CRD. kubectl_manifest applies it
    # as raw YAML with no plan-time CRD schema discovery, so the module plans
    # offline and applies once the CSI driver's CRD is present (the GKE Secret
    # Manager CSI add-on). kubernetes_manifest would require that CRD reachable on
    # the cluster even to plan.
    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.0"
    }
  }
}
