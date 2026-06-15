# Proves the kms module resolves a key to the same output interface (key_arn,
# key_id, alias_name) whether it CREATES a CMK (provision) or RESOLVES a
# customer-supplied key (byo) — the BYOC path where a customer already manages
# their own CMK. command = plan; the BYO data source is mocked so no AWS account
# is needed.
#
# BYO inputs use the alias form (alias/<name>): the AWS provider's aws_kms_key
# data-source argument validator runs even under mock_provider and, in the test
# framework, accepts the alias form while rejecting a bare key-id/ARN. A key
# alias is a valid, realistic BYO reference, so the tests supply one.

mock_provider "aws" {
  mock_data "aws_kms_key" {
    defaults = {
      arn     = "arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
      key_id  = "1234abcd-12ab-34cd-56ef-1234567890ab"
      enabled = true
    }
  }
}

run "provision_creates_cmk" {
  command = plan

  variables {
    mode  = "provision"
    alias = "workload-cmk"
  }

  assert {
    condition     = length(aws_kms_key.this) == 1
    error_message = "provision mode must create exactly one CMK."
  }
  assert {
    condition     = length(aws_kms_alias.this) == 1
    error_message = "provision mode must create the alias."
  }
  assert {
    condition     = length(data.aws_kms_key.byo) == 0
    error_message = "provision mode must NOT look up a BYO key."
  }
  assert {
    condition     = aws_kms_key.this[0].enable_key_rotation == true
    error_message = "provision mode must enable key rotation by default."
  }
  assert {
    condition     = output.alias_name == "alias/workload-cmk"
    error_message = "provision mode must expose the alias name."
  }
}

run "byo_resolves_existing_key" {
  command = plan

  variables {
    mode = "byo"
    # The BYOC case: a customer who already manages their own CMK (referenced by
    # its alias) and wants every consumer to encrypt under it.
    provided_key_arn = "alias/customer-cmk"
  }

  assert {
    condition     = length(aws_kms_key.this) == 0
    error_message = "byo mode must NOT create a CMK."
  }
  assert {
    condition     = length(aws_kms_alias.this) == 0
    error_message = "byo mode must NOT create an alias."
  }
  assert {
    condition     = length(data.aws_kms_key.byo) == 1
    error_message = "byo mode must look up exactly the supplied key."
  }
  assert {
    condition     = output.key_arn == "arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
    error_message = "byo mode must expose the resolved key ARN under the same output key as provision mode."
  }
  assert {
    condition     = output.alias_name == ""
    error_message = "byo mode alias_name must be an empty string (same type as provision mode)."
  }
}

# A disabled BYO key must fail the plan-time precondition rather than silently
# resolving an unusable key.
run "byo_disabled_key_is_rejected" {
  command = plan

  variables {
    mode             = "byo"
    provided_key_arn = "alias/customer-disabled-cmk"
  }

  override_data {
    target = data.aws_kms_key.byo[0]
    values = {
      arn     = "arn:aws:kms:us-east-1:111122223333:key/dddddddd-dddd-dddd-dddd-dddddddddddd"
      key_id  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
      enabled = false
    }
  }

  expect_failures = [
    terraform_data.key_usable,
  ]
}
