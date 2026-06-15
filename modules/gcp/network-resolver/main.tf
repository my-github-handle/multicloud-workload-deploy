locals {
  is_byo = var.mode == "byo"
}

# BYO lookups only run in byo mode (count gates them off in provision mode).
# This is the SINGLE create-vs-lookup branch in the whole GCP network path —
# the single branch lives only in the resolver; everything downstream receives
# identical inputs.
data "google_compute_network" "byo" {
  count   = local.is_byo ? 1 : 0
  name    = var.byo_network_name
  project = var.project_id
}

data "google_compute_subnetwork" "byo" {
  count   = local.is_byo ? 1 : 0
  name    = var.byo_subnet_name
  project = var.project_id
  region  = var.region
}

locals {
  # Coalesce both modes into one uniform interface. vpc_id is the network
  # self-link (the stable cross-module identifier on GCP); subnet_ids is a list
  # of subnet self-links.
  resolved_vpc_id = local.is_byo ? data.google_compute_network.byo[0].self_link : var.provisioned_network_self_link

  resolved_subnet_ids = local.is_byo ? [data.google_compute_subnetwork.byo[0].self_link] : var.provisioned_subnet_self_links

  resolved_egress_path_ref = local.is_byo ? var.byo_egress_path_ref : var.provisioned_egress_path_ref
}
