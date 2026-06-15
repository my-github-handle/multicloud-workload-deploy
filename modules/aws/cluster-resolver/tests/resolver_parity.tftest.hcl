# Asserts provision-mode and BYO-mode produce an IDENTICAL output interface
# ({endpoint (full https://), ca, auth (tagged exec object)}) — the create-vs-
# lookup parity guarantee. command = plan; the BYO lookup is overridden so no AWS
# account is needed.

run "provision_passes_cluster_outputs_through" {
  command = plan

  variables {
    mode                 = "provision"
    cluster_name         = "demo"
    provisioned_endpoint = "https://ABC123.gr7.us-east-1.eks.amazonaws.com"
    provisioned_ca       = "LS0tLS1CRUdJTi1mYWtlLWNh"
  }

  assert {
    condition     = output.endpoint == "https://ABC123.gr7.us-east-1.eks.amazonaws.com"
    error_message = "provision mode must pass the cluster endpoint through as a full https:// URL."
  }
  assert {
    condition     = output.ca == "LS0tLS1CRUdJTi1mYWtlLWNh"
    error_message = "provision mode must pass the cluster CA through."
  }
  # auth is the tagged exec object — no static token (avoids token churn).
  assert {
    condition     = output.auth.kind == "exec"
    error_message = "auth must default to the EKS exec-plugin form."
  }
  assert {
    condition     = output.auth.token == null
    error_message = "the exec auth form must carry no static token."
  }
  assert {
    condition     = output.auth.exec.command == "aws" && contains(output.auth.exec.args, "get-token")
    error_message = "the exec auth must invoke `aws eks get-token`."
  }
  assert {
    condition     = contains(output.auth.exec.args, "demo")
    error_message = "the exec auth must target the named cluster."
  }
}

run "byo_normalizes_bare_host_endpoint" {
  command = plan

  variables {
    mode         = "byo"
    cluster_name = "customer-existing"
  }

  # The BYOC case: a customer's existing EKS cluster we deploy into. A bare host
  # (no scheme) must be normalized to a full https:// URL inside the resolver.
  override_data {
    target = data.aws_eks_cluster.byo[0]
    values = {
      endpoint              = "ABC999.gr7.us-east-1.eks.amazonaws.com"
      certificate_authority = [{ data = "LS0tLS1CRUdJTi1ieW8tY2E=" }]
    }
  }

  assert {
    condition     = output.endpoint == "https://ABC999.gr7.us-east-1.eks.amazonaws.com"
    error_message = "BYO mode must normalize a bare-host endpoint to a full https:// URL."
  }
  assert {
    condition     = output.ca == "LS0tLS1CRUdJTi1ieW8tY2E="
    error_message = "BYO mode must expose the looked-up CA under the same output key."
  }
  # Same tagged auth shape as provision mode, targeting the BYO cluster.
  assert {
    condition     = output.auth.kind == "exec" && contains(output.auth.exec.args, "customer-existing")
    error_message = "BYO mode must emit the same exec auth shape, targeting the BYO cluster."
  }
}
