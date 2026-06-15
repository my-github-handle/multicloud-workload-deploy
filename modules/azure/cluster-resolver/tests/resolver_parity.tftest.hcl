# The cluster-resolver `auth` is a tagged object with a stable shape across
# provision/BYO and client_cert/exec. command = plan; the BYO data source is
# mocked so no Azure subscription is needed.

mock_provider "azurerm" {
  mock_data "azurerm_kubernetes_cluster" {
    defaults = {
      kube_config = [{
        host                   = "https://byo-aks-0000.hcp.eastus.azmk8s.io:443"
        cluster_ca_certificate = "BYO_CA"
        client_certificate     = "" # local_account_disabled => empty
        client_key             = ""
      }]
    }
  }
}

run "provision_exec_auth_shape" {
  command = plan
  variables {
    mode                  = "provision"
    auth_mode             = "exec"
    resource_group_name   = "prov-rg"
    cluster_name_for_exec = "prov-aks"
    provisioned_host      = "https://prov-aks-0000.hcp.eastus.azmk8s.io:443"
    provisioned_ca        = "PROV_CA"
  }
  assert {
    condition     = output.auth.mode == "exec"
    error_message = "provision+exec must tag auth.mode = exec."
  }
  assert {
    condition     = output.auth.client_certificate == "" && output.auth.client_key == ""
    error_message = "exec mode must leave client cert/key empty."
  }
  assert {
    condition     = output.auth.exec.cluster_name == "prov-aks"
    error_message = "exec auth must carry the cluster name for kubelogin."
  }
}

run "byo_exec_auth_shape" {
  command = plan
  variables {
    mode                  = "byo"
    auth_mode             = "exec"
    resource_group_name   = "byo-rg"
    provided_cluster_name = "byo-aks"
  }
  # Same tag, identical shape to provision mode.
  assert {
    condition     = output.auth.mode == "exec" && output.auth.exec.cluster_name == "byo-aks"
    error_message = "BYO+exec must emit the same tagged exec auth shape as provision mode."
  }
}

run "provision_client_cert_auth_shape" {
  command = plan
  variables {
    mode                           = "provision"
    auth_mode                      = "client_cert"
    resource_group_name            = "prov-rg"
    provisioned_host               = "https://prov-aks-0000.hcp.eastus.azmk8s.io:443"
    provisioned_ca                 = "PROV_CA"
    provisioned_client_certificate = "PROV_CERT"
    provisioned_client_key         = "PROV_KEY"
  }
  assert {
    condition     = output.auth.mode == "client_cert" && output.auth.client_certificate == "PROV_CERT"
    error_message = "client_cert mode must carry the cert/key pair."
  }
}
