# Plan-time assertions for the secrets module: secrets are CMK-encrypted with the
# resolved key, the SecretProviderClass is rendered only when enabled, and the
# greenfield Phase-1 toggle (create_secret_provider_class = false) still creates
# the secrets without the CSI manifest. command = plan; no AWS account needed.
#
# The module declares no provider blocks (the root supplies them); a terraform test
# run must, so the kubectl provider can initialize without contacting a cluster.
provider "kubectl" {
  load_config_file = false
  host             = "https://127.0.0.1:1"
}

variables {
  name        = "demo"
  namespace   = "workload-system"
  region      = "us-east-1"
  kms_key_arn = "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
  secrets = {
    "db-password" = "initial-value-rotate-me"
    "api-token"   = "initial-token-rotate-me"
  }
}

run "secrets_are_cmk_encrypted_and_spc_rendered" {
  command = plan

  variables {
    create_secret_provider_class = true
  }

  assert {
    condition     = length(aws_secretsmanager_secret.this) == 2
    error_message = "one Secrets Manager secret must be created per entry in var.secrets."
  }
  assert {
    condition     = alltrue([for s in aws_secretsmanager_secret.this : s.kms_key_id == var.kms_key_arn])
    error_message = "every secret must be envelope-encrypted with the resolved CMK ARN."
  }
  assert {
    condition     = aws_secretsmanager_secret.this["db-password"].name == "demo-db-password"
    error_message = "secret names must be prefixed with the module name."
  }
  assert {
    condition     = length(kubectl_manifest.secret_provider_class) == 1
    error_message = "the SecretProviderClass must be rendered when create_secret_provider_class = true."
  }
  assert {
    condition     = output.secret_provider_class_name == "demo-secrets"
    error_message = "secret_provider_class_name output must report the SPC name when enabled."
  }
  # secrets_ref mirrors the SPC name when enabled; assert it stays consistent.
  assert {
    condition     = output.secrets_ref == output.secret_provider_class_name
    error_message = "secrets_ref must equal secret_provider_class_name (the mounted SPC) when enabled."
  }
}

run "phase1_creates_secrets_without_csi_manifest" {
  command = plan

  # Greenfield Phase 1 / BYOC before the CSI CRD exists: the secrets are still
  # created, but the SecretProviderClass is not applied.
  variables {
    create_secret_provider_class = false
  }

  assert {
    condition     = length(aws_secretsmanager_secret.this) == 2
    error_message = "secrets must still be created when the SPC is disabled."
  }
  assert {
    condition     = length(kubectl_manifest.secret_provider_class) == 0
    error_message = "no SecretProviderClass must be rendered when create_secret_provider_class = false."
  }
  assert {
    condition     = output.secret_provider_class_name == ""
    error_message = "secret_provider_class_name must be empty when the SPC is disabled."
  }
  assert {
    condition     = output.secrets_ref == ""
    error_message = "secrets_ref must be empty when the SPC is disabled."
  }
}
