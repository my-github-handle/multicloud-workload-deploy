# Each secret is envelope-encrypted with the resolved CMK.
resource "aws_secretsmanager_secret" "this" {
  for_each = var.secrets

  name       = "${var.name}-${each.key}"
  kms_key_id = var.kms_key_arn
  tags       = var.tags
}

resource "aws_secretsmanager_secret_version" "this" {
  for_each = var.secrets

  secret_id     = aws_secretsmanager_secret.this[each.key].id
  secret_string = each.value
}

locals {
  secret_arns = [for s in aws_secretsmanager_secret.this : s.arn]

  # Secrets Store CSI driver wiring: a SecretProviderClass referencing the AWS
  # provider so the workload pod (with the IRSA role) mounts the secrets at
  # runtime. The CSI driver + AWS provider DaemonSet are installed at cluster
  # bootstrap; this renders the per-workload SecretProviderClass.
  spc_manifest = {
    apiVersion = "secrets-store.csi.x-k8s.io/v1"
    kind       = "SecretProviderClass"
    metadata = {
      name      = "${var.name}-secrets"
      namespace = var.namespace
    }
    spec = {
      provider = "aws"
      parameters = {
        region = var.region
        objects = yamlencode([
          for k, s in aws_secretsmanager_secret.this : {
            objectName  = s.arn
            objectType  = "secretsmanager"
            objectAlias = k
          }
        ])
      }
    }
  }
}

# kubectl_manifest (raw YAML) — not kubernetes_manifest — so no plan-time CRD
# schema discovery is needed (see versions.tf).
resource "kubectl_manifest" "secret_provider_class" {
  count = var.create_secret_provider_class ? 1 : 0

  yaml_body         = yamlencode(local.spc_manifest)
  server_side_apply = true
}
