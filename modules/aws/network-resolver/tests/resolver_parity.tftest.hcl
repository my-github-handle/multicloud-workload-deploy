# Asserts provision-mode and BYO-mode produce an IDENTICAL output interface
# ({vpc_id (string), subnet_ids (list), pod_subnet_ids (list), egress_path_ref
# (string)}) — the create-vs-lookup parity guarantee. command = plan; the BYO
# data sources are mocked so no AWS account is needed.

mock_provider "aws" {
  mock_data "aws_vpc" {
    defaults = {
      id = "vpc-byomock00000000"
    }
  }
  mock_data "aws_subnets" {
    defaults = {
      ids = ["subnet-byomock0000001", "subnet-byomock0000002"]
    }
  }
}

run "provision_mode_outputs" {
  command = plan

  variables {
    mode                        = "provision"
    provisioned_vpc_id          = "vpc-provisioned00001"
    provisioned_subnet_ids      = ["subnet-prov00000001", "subnet-prov00000002"]
    provisioned_pod_subnet_ids  = ["subnet-prov0000pod1", "subnet-prov0000pod2"]
    provisioned_egress_path_ref = "arn:aws:network-firewall:us-east-1:111122223333:firewall/demo-egress-fw"
  }

  assert {
    condition     = output.vpc_id == "vpc-provisioned00001"
    error_message = "provision mode must pass the provisioned VPC ID straight through."
  }
  assert {
    condition     = length(output.subnet_ids) == 2
    error_message = "provision mode must expose the provisioned node subnet IDs."
  }
  assert {
    condition     = length(output.pod_subnet_ids) == 2
    error_message = "provision mode must expose the provisioned pod subnet IDs."
  }
  assert {
    condition     = output.egress_path_ref == "arn:aws:network-firewall:us-east-1:111122223333:firewall/demo-egress-fw"
    error_message = "provision mode must pass the provisioned egress path ref through."
  }
}

run "byo_mode_outputs" {
  command = plan

  variables {
    mode                      = "byo"
    byo_vpc_id                = "vpc-byomock00000000"
    byo_subnet_tag_filter     = { "kubernetes.io/role/internal-elb" = "1" }
    byo_pod_subnet_tag_filter = { "kubernetes.io/role/cni" = "1" }
    byo_egress_path_ref       = ""
  }

  # Same output keys, same types, populated from the looked-up VPC/subnets.
  assert {
    condition     = output.vpc_id == "vpc-byomock00000000"
    error_message = "BYO mode must expose the looked-up VPC ID under the same output key."
  }
  assert {
    condition     = length(output.subnet_ids) == 2
    error_message = "BYO mode must expose the looked-up node subnet IDs as a list, same shape as provision mode."
  }
  assert {
    condition     = length(output.pod_subnet_ids) == 2
    error_message = "BYO mode must expose the looked-up pod subnet IDs as a list when a pod tag filter is given."
  }
  assert {
    condition     = output.egress_path_ref == ""
    error_message = "BYO egress_path_ref must still be a string (empty when the customer owns the edge firewall)."
  }
}

run "byo_mode_without_pod_subnets" {
  command = plan

  variables {
    mode                      = "byo"
    byo_vpc_id                = "vpc-byomock00000000"
    byo_subnet_tag_filter     = { "kubernetes.io/role/internal-elb" = "1" }
    byo_pod_subnet_tag_filter = {}
    byo_egress_path_ref       = "arn:aws:network-firewall:us-east-1:111122223333:firewall/customer-fw"
  }

  # When no pod tag filter is given, pods share the node subnets: pod_subnet_ids
  # is an empty list (still a list — same shape), not null.
  assert {
    condition     = length(output.pod_subnet_ids) == 0
    error_message = "BYO mode without a pod tag filter must expose an empty pod_subnet_ids list (pods share node subnets)."
  }
  assert {
    condition     = output.egress_path_ref == "arn:aws:network-firewall:us-east-1:111122223333:firewall/customer-fw"
    error_message = "BYO mode must pass a customer-supplied egress path ref through."
  }
}
