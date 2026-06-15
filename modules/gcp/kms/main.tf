locals {
  is_provision = var.mode == "provision"
  is_byo       = var.mode == "byo"
}

# Provision: a customer-managed KeyRing + CryptoKey with rotation.
resource "google_kms_key_ring" "this" {
  count    = local.is_provision ? 1 : 0
  name     = var.key_ring_name
  project  = var.project_id
  location = var.region
}

resource "google_kms_crypto_key" "this" {
  count           = local.is_provision ? 1 : 0
  name            = var.crypto_key_name
  key_ring        = google_kms_key_ring.this[0].id
  rotation_period = var.rotation_period
  purpose         = "ENCRYPT_DECRYPT"
  labels          = var.labels

  lifecycle {
    prevent_destroy = true # KMS keys cannot be truly deleted; guard against accidental destroy.
    # Teardown note: prevent_destroy makes `terraform destroy` ERROR on this
    # resource. Before teardown, either (a) remove this lifecycle block and
    # re-apply, or (b) `terraform destroy` with this resource excluded
    # (destroy everything else, leave the key). KMS keys are scheduled for
    # destruction, not deleted immediately, even once prevent_destroy is lifted.
  }
}

# BYO: resolve a supplied CryptoKey and verify it is usable. The resource id form
# is projects/P/locations/L/keyRings/R/cryptoKeys/K.
data "google_kms_crypto_key" "byo" {
  count    = local.is_byo ? 1 : 0
  name     = element(split("/", var.provided_key_id), length(split("/", var.provided_key_id)) - 1)
  key_ring = join("/", slice(split("/", var.provided_key_id), 0, 6))
}

locals {
  resolved_key_id = local.is_provision ? google_kms_crypto_key.this[0].id : data.google_kms_crypto_key.byo[0].id
}
