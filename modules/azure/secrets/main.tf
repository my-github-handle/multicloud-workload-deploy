# Each secret is stored in the resolved Key Vault, which encrypts its contents at
# rest with the vault's key. Rotate the initial value out-of-band after apply.
resource "azurerm_key_vault_secret" "this" {
  for_each = var.secrets

  name         = "${var.name}-${each.key}"
  value        = each.value
  key_vault_id = var.key_vault_id
  tags         = var.tags
}

locals {
  secret_ids = [for s in azurerm_key_vault_secret.this : s.id]

  # One CSI object per secret, keyed by config (var.secrets) so the rendered
  # `objects` string resolves at plan time, not from a computed resource attr.
  csi_objects = [
    for k, _v in var.secrets : {
      objectName = "${var.name}-${k}"
      objectType = "secret"
    }
  ]

  # The Azure Key Vault provider's `objects` parameter is a SINGLE YAML document
  # string whose top-level `array` key holds a list of per-object YAML strings:
  #     array:
  #       - |
  #         objectName: foo
  #         objectType: secret
  # One yamlencode per element makes each a YAML scalar; one outer yamlencode
  # renders the wrapper. Encoding the whole thing twice produces malformed nested
  # YAML the CSI driver rejects only at pod-mount time (it passes plan/validate).
  csi_objects_yaml = yamlencode({
    array = [for o in local.csi_objects : yamlencode(o)]
  })

  spc_manifest = {
    apiVersion = "secrets-store.csi.x-k8s.io/v1"
    kind       = "SecretProviderClass"
    metadata = {
      name      = "${var.name}-secrets"
      namespace = var.namespace
    }
    spec = {
      provider = "azure"
      parameters = {
        usePodIdentity       = "false"
        useVMManagedIdentity = "false"
        clientID             = var.uami_client_id
        keyvaultName         = var.key_vault_name
        tenantId             = var.tenant_id
        objects              = local.csi_objects_yaml
      }
    }
  }
}

# SecretProviderClass (azure provider): the workload pod, running as the UAMI via
# Workload Identity, mounts the Key Vault secrets at runtime. The CSI driver +
# provider come from the AKS azure-keyvault-secrets-provider add-on (cluster
# module); this renders the per-workload SPC. Applied as raw YAML so no plan-time
# CRD schema discovery is needed.
resource "kubectl_manifest" "secret_provider_class" {
  count = var.create_secret_provider_class ? 1 : 0

  yaml_body         = yamlencode(local.spc_manifest)
  server_side_apply = true
}
