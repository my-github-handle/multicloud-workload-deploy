locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Provision: create a dedicated project for the deployment. On GCP the project is
# the fundamental container — billing link, IAM boundary, and the scope service
# APIs are enabled on. A dedicated project keeps the workload's blast radius and
# quota isolated from the rest of the customer's estate.
resource "google_project" "this" {
  count = local.is_provision ? 1 : 0

  project_id      = var.project_id
  name            = var.project_name != "" ? var.project_name : var.project_id
  billing_account = var.billing_account != "" ? var.billing_account : null
  org_id          = var.org_id != "" ? var.org_id : null
  folder_id       = var.folder_id != "" ? var.folder_id : null
  labels          = var.labels

  # Do NOT create the default network — it ships permissive firewall rules
  # (broad ingress). The network module provisions a hardened VPC instead.
  auto_create_network = false

  # Keep the project if the deployment is torn down but the customer still wants
  # the container; the building-block teardown removes resources, not the project.
  deletion_policy = "PREVENT"
}

# BYO (the BYOC path): resolve an existing customer project. We do not create or
# move it — the customer owns its lifecycle, billing, and parent.
data "google_project" "byo" {
  count      = local.is_byo ? 1 : 0
  project_id = var.project_id
}

locals {
  resolved_project_id     = local.is_provision ? google_project.this[0].project_id : data.google_project.byo[0].project_id
  resolved_project_number = local.is_provision ? google_project.this[0].number : data.google_project.byo[0].number
}

# Enable the required service APIs in BOTH modes. On a provisioned project this is
# the initial enablement; on a BYO project this brings the customer's project up
# to the baseline the building blocks need (idempotent — already-enabled APIs are
# a no-op). disable_dependent_services stays false so we never cascade-disable an
# API a sibling workload in a BYO project depends on.
resource "google_project_service" "required" {
  for_each = toset(var.activate_apis)

  project = local.resolved_project_id
  service = each.value

  disable_on_destroy         = var.disable_services_on_destroy
  disable_dependent_services = false
}
