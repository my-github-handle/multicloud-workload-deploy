# Terraform-native, co-located GCP data-source pre-checks. These complement (do
# NOT duplicate) the Go cloud.Provider staged checks: these run inside the plan
# graph and fail fast on resolved-resource sanity; the Go provider does the
# permission/egress/identity simulation surfaced through the binary + report.

data "google_project" "current" {
  project_id = var.project_id
}

data "google_compute_network" "resolved" {
  name    = element(split("/", var.network_self_link), length(split("/", var.network_self_link)) - 1)
  project = var.project_id
}

data "google_kms_crypto_key" "resolved" {
  name     = element(split("/", var.kms_key_id), length(split("/", var.kms_key_id)) - 1)
  key_ring = join("/", slice(split("/", var.kms_key_id), 0, 6))
}

# Project resolves (number is non-empty).
resource "terraform_data" "project_resolves" {
  input = var.project_id
  lifecycle {
    precondition {
      condition     = data.google_project.current.number != ""
      error_message = "Project ${var.project_id} did not resolve to a project number."
    }
  }
}

# Network resolves (self_link matches the input).
resource "terraform_data" "network_resolves" {
  input = var.network_self_link
  lifecycle {
    precondition {
      condition     = data.google_compute_network.resolved.self_link != ""
      error_message = "Resolved VPC network ${var.network_self_link} did not resolve."
    }
  }
}

# CryptoKey resolves (id matches the input).
resource "terraform_data" "kms_resolves" {
  input = var.kms_key_id
  lifecycle {
    precondition {
      condition     = data.google_kms_crypto_key.resolved.id != ""
      error_message = "Resolved CryptoKey ${var.kms_key_id} did not resolve."
    }
  }
}
