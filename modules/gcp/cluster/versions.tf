terraform {
  required_version = ">= 1.7.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    # google-beta is required ONLY for advanced_datapath_observability_config
    # (Dataplane V2 / Hubble-grade observability). The rest of the module uses the
    # GA google provider.
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 6.0"
    }
  }
}
