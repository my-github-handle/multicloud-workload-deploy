locals {
  is_byo = var.mode == "byo"
}

# BYO: look up the existing cluster's endpoint + CA.
data "google_container_cluster" "byo" {
  count    = local.is_byo ? 1 : 0
  name     = var.cluster_name
  location = var.location
  project  = var.project_id
}

# Auth token comes from the Google client config in BOTH modes (it is short-lived
# and not a provisioning output) so the kubernetes/helm providers in Layer 3
# authenticate identically whether the cluster was created or looked up — the GKE
# token-auth model (gke-gcloud-auth-plugin equivalent).
data "google_client_config" "current" {}

locals {
  # Raw endpoint host from either mode (GKE outputs a bare host with NO scheme,
  # but a BYO value might already carry https://). Normalize: STRIP any existing
  # scheme, then prefix https:// exactly once — fixes the double-prefix risk
  # (https://https://…).
  raw_endpoint = local.is_byo ? data.google_container_cluster.byo[0].endpoint : var.provisioned_endpoint
  # Strip a leading http:// or https:// if present, then re-prefix exactly once.
  # The regex delimiter is /…/; the literal scheme slashes are matched as [/] to
  # avoid colliding with the delimiter.
  bare_endpoint = replace(local.raw_endpoint, "/^https?:[/][/]/", "")

  resolved_endpoint = "https://${local.bare_endpoint}"
  resolved_ca       = local.is_byo ? data.google_container_cluster.byo[0].master_auth[0].cluster_ca_certificate : var.provisioned_ca
  resolved_token    = data.google_client_config.current.access_token
}
