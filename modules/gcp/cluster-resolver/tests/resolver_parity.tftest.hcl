# Asserts provision-mode and BYO-mode produce an IDENTICAL {endpoint, ca, auth}
# interface, that endpoint is normalized to a single https:// prefix, and that
# the token-form auth is populated. command = plan; all data sources mocked.

mock_provider "google" {
  mock_data "google_container_cluster" {
    defaults = {
      endpoint    = "10.0.0.2"
      master_auth = [{ cluster_ca_certificate = "QkFTRTY0Q0E=" }]
    }
  }
  mock_data "google_client_config" {
    defaults = {
      access_token = "ya29.mock-access-token"
    }
  }
}

run "provision_mode_outputs" {
  command = plan

  variables {
    mode                 = "provision"
    project_id           = "demo"
    location             = "us-central1"
    cluster_name         = "demo"
    provisioned_endpoint = "10.0.0.2" # bare host, no scheme (as GKE emits)
    provisioned_ca       = "QkFTRTY0Q0E="
  }

  assert {
    condition     = output.endpoint == "https://10.0.0.2"
    error_message = "provision mode must emit a single-https://-prefixed endpoint (scheme normalization)."
  }
  assert {
    condition     = output.auth != ""
    error_message = "auth (token form) must be populated from data.google_client_config."
  }
}

run "byo_mode_outputs" {
  command = plan

  variables {
    mode         = "byo"
    project_id   = "demo"
    location     = "us-central1"
    cluster_name = "demo"
  }

  # Same three output keys, same types, https:// prefixed exactly once even if the
  # looked-up endpoint already carried a scheme.
  assert {
    condition     = startswith(output.endpoint, "https://") && length(regexall("https://https://", output.endpoint)) == 0
    error_message = "BYO mode endpoint must be https:// prefixed exactly once (no double-prefix)."
  }
  assert {
    condition     = output.ca == "QkFTRTY0Q0E="
    error_message = "BYO mode must expose the looked-up CA under the same key as provision mode."
  }
  assert {
    condition     = output.auth != ""
    error_message = "BYO mode auth (token form) must be populated identically to provision mode."
  }
}
