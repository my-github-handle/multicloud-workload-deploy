output "key_id" {
  description = "Resolved CryptoKey resource id (created or BYO), e.g. projects/P/locations/L/keyRings/R/cryptoKeys/K."
  value       = local.resolved_key_id
}

output "key_ring_id" {
  description = "KeyRing id in provision mode; empty in BYO mode."
  value       = local.is_provision ? google_kms_key_ring.this[0].id : ""
}

output "crypto_key_name" {
  description = "CryptoKey short name in provision mode; empty in BYO mode."
  value       = local.is_provision ? google_kms_crypto_key.this[0].name : ""
}
