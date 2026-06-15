# Each secret is CMEK-encrypted with the resolved Cloud KMS key. A user-managed
# regional replica pins the secret to the key's region and lets us attach the
# customer_managed_encryption block.
resource "google_secret_manager_secret" "this" {
  for_each = var.secrets

  secret_id = "${var.name}-${each.key}"
  project   = var.project_id

  replication {
    user_managed {
      replicas {
        location = var.region
        customer_managed_encryption {
          kms_key_name = var.kms_key_id
        }
      }
    }
  }
}

resource "google_secret_manager_secret_version" "this" {
  for_each = var.secrets

  secret      = google_secret_manager_secret.this[each.key].id
  secret_data = each.value
}

locals {
  secret_ids = [for s in google_secret_manager_secret.this : s.id]
}

# Secrets Store CSI driver wiring: a SecretProviderClass referencing the GCP
# provider so the workload pod (via Workload Identity) mounts the secrets at
# runtime. The CSI driver + GCP provider DaemonSet are installed in Layer 3 /
# cluster bootstrap; this renders the per-workload SPC.
#
# kubectl_manifest (raw YAML) — NOT kubernetes_manifest — so no plan-time CRD
# schema discovery is needed (see versions.tf). Built from a local so the output
# can read the name without a resource attribute.
locals {
  spc_manifest = {
    apiVersion = "secrets-store.csi.x-k8s.io/v1"
    kind       = "SecretProviderClass"
    metadata = {
      name      = "${var.name}-secrets"
      namespace = var.namespace
    }
    spec = {
      provider = "gcp"
      parameters = {
        secrets = yamlencode([
          for k, s in google_secret_manager_secret.this : {
            resourceName = "${s.id}/versions/latest"
            path         = k
          }
        ])
      }
    }
  }
}

resource "kubectl_manifest" "secret_provider_class" {
  count = var.create_secret_provider_class ? 1 : 0

  yaml_body         = yamlencode(local.spc_manifest)
  server_side_apply = true
}
