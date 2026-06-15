# Azure preflight: the two halves

The Azure preflight contract has two co-located parts:

1. **Go `cloud.Provider` (the real staged cloud checks).**
   `operator/internal/cloud/azure` implements the four `cloud.Provider` methods —
   `CheckIdentityPermissions` (Stage 0), `CheckKMSKey` (Stage 1),
   `CheckSecretsBackend` (Stage 2), `CheckEgress` (Stage 3) — using the
   `github.com/Azure/azure-sdk-for-go/sdk` packages. The preflight binary
   (`operator/cmd/preflight`) selects this provider via `--cloud=azure`
   (`selectProvider("azure")`), runs the staged Runner, and emits the
   green/amber/red report. The Layer-3 `modules/preflight` invokes the binary and
   gates `terraform apply` on the verdict. **This is where the role-assignment
   permission check, Key Vault key state, secret-in-vault, and egress-posture
   checks live.**

2. **This Terraform module (co-located data-source sanity pre-checks).**
   `checks.tf` asserts, inside the plan graph, that the *resolved* resources are
   coherent: the VNet region matches and the Key Vault has purge protection.
   These are cheap precondition guards that fail the plan fast and live next to
   the modules that produce the inputs. They do **not** re-implement the staged
   report — they backstop it with graph-time assertions.

In `live/azure-full`, the binary runs with `--mode=full --cloud=azure`: stages the
greenfield path satisfies *by provisioning* are informational (downgraded
red→amber), not blocking. This module's preconditions still hard-fail on a
genuinely broken resolved resource.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.7.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | 4.77.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | 4.77.0 |
| <a name="provider_terraform"></a> [terraform](#provider\_terraform) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [terraform_data.key_present](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.kv_purge_protection](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [terraform_data.region_match](https://registry.terraform.io/providers/hashicorp/terraform/latest/docs/resources/data) | resource |
| [azurerm_client_config.current](https://registry.terraform.io/providers/hashicorp/azurerm/4.77.0/docs/data-sources/client_config) | data source |
| [azurerm_key_vault.resolved](https://registry.terraform.io/providers/hashicorp/azurerm/4.77.0/docs/data-sources/key_vault) | data source |
| [azurerm_virtual_network.resolved](https://registry.terraform.io/providers/hashicorp/azurerm/4.77.0/docs/data-sources/virtual_network) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_key_id"></a> [key\_id](#input\_key\_id) | Resolved Key Vault Key ID to sanity-check (must be enabled). | `string` | n/a | yes |
| <a name="input_key_vault_id"></a> [key\_vault\_id](#input\_key\_vault\_id) | Resolved Key Vault ID to sanity-check (must have purge protection). | `string` | n/a | yes |
| <a name="input_location"></a> [location](#input\_location) | Azure region (asserted to match the resolved VNet's location). | `string` | n/a | yes |
| <a name="input_vnet_id"></a> [vnet\_id](#input\_vnet\_id) | Resolved VNet ID to sanity-check (must exist). | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_checks_passed"></a> [checks\_passed](#output\_checks\_passed) | True once all co-located Azure data-source preconditions have evaluated (region/VNet/Key Vault). |
| <a name="output_subscription_id"></a> [subscription\_id](#output\_subscription\_id) | The Azure subscription the deploy is running against (for the report/logs). |
<!-- END_TF_DOCS -->
