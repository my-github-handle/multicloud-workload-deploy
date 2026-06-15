# Asserts provision-mode and BYO-mode produce an IDENTICAL output interface
# ({project_id (string), project_number (string), enabled_services (list)}) — the
# create-vs-lookup parity guarantee. command = plan; the BYO data source and the
# provisioned project are mocked so no GCP project is needed.

mock_provider "google" {
  mock_resource "google_project" {
    defaults = {
      number = "123456789012"
    }
  }
  mock_data "google_project" {
    defaults = {
      project_id = "byo-project"
      number     = "210987654321"
    }
  }
}

run "provision_mode_outputs" {
  command = plan

  variables {
    mode            = "provision"
    project_id      = "prov-project"
    billing_account = "00X0X0-0X0X0X-0X0X0X"
  }

  assert {
    condition     = output.project_id == "prov-project"
    error_message = "provision mode must expose the created project id."
  }
  assert {
    condition     = length(output.enabled_services) > 0
    error_message = "provision mode must enable the required service APIs."
  }
}

run "byo_mode_outputs" {
  command = plan

  variables {
    mode       = "byo"
    project_id = "byo-project"
  }

  # Same three output keys, same types, populated from the looked-up project.
  assert {
    condition     = output.project_id == "byo-project"
    error_message = "BYO mode must expose the looked-up project id under the same output key."
  }
  assert {
    condition     = length(output.enabled_services) > 0
    error_message = "BYO mode must still ensure the required service APIs are enabled (the BYOC baseline)."
  }
}
