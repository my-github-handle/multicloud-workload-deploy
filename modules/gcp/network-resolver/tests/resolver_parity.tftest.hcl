# Asserts provision-mode and BYO-mode produce an IDENTICAL output interface
# ({vpc_id (string), subnet_ids (list), egress_path_ref (string)}) — the
# create-vs-lookup parity guarantee. command = plan; BYO data sources are mocked
# so no GCP project is needed.

mock_provider "google" {
  mock_data "google_compute_network" {
    defaults = {
      self_link = "https://www.googleapis.com/compute/v1/projects/demo/global/networks/byo-vpc"
    }
  }
  mock_data "google_compute_subnetwork" {
    defaults = {
      self_link = "https://www.googleapis.com/compute/v1/projects/demo/regions/us-central1/subnetworks/byo-subnet"
    }
  }
}

run "provision_mode_outputs" {
  command = plan

  variables {
    mode                          = "provision"
    project_id                    = "demo"
    provisioned_network_self_link = "https://www.googleapis.com/compute/v1/projects/demo/global/networks/prov-vpc"
    provisioned_subnet_self_links = ["https://www.googleapis.com/compute/v1/projects/demo/regions/us-central1/subnetworks/prov-subnet"]
    provisioned_egress_path_ref   = "demo-egress-policy"
  }

  assert {
    condition     = output.vpc_id == "https://www.googleapis.com/compute/v1/projects/demo/global/networks/prov-vpc"
    error_message = "provision mode must pass the provisioned network self-link straight through."
  }
  assert {
    condition     = length(output.subnet_ids) == 1
    error_message = "provision mode must expose the provisioned subnet self-links."
  }
  assert {
    condition     = output.egress_path_ref == "demo-egress-policy"
    error_message = "provision mode must pass the provisioned egress path ref through."
  }
}

run "byo_mode_outputs" {
  command = plan

  variables {
    mode                = "byo"
    project_id          = "demo"
    region              = "us-central1"
    byo_network_name    = "byo-vpc"
    byo_subnet_name     = "byo-subnet"
    byo_egress_path_ref = ""
  }

  # Same three output keys, same types, populated from the looked-up VPC/subnet.
  assert {
    condition     = output.vpc_id == "https://www.googleapis.com/compute/v1/projects/demo/global/networks/byo-vpc"
    error_message = "BYO mode must expose the looked-up network self-link under the same output key."
  }
  assert {
    condition     = length(output.subnet_ids) == 1
    error_message = "BYO mode must expose the looked-up subnet as a list, same shape as provision mode."
  }
  assert {
    condition     = output.egress_path_ref == ""
    error_message = "BYO mode egress_path_ref must still be a string (empty when the customer owns the edge firewall)."
  }
}
