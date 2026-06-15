# Proves byo-identity mode (the BYOC case: a customer who creates the IRSA role
# themselves and wants us to deploy against it). The module must NOT create a
# role, must resolve the supplied role ARN, and must STILL render the reviewable
# policy + trust artifacts so the customer can attach them. command = plan; the
# BYO role lookup is overridden so no AWS account is needed.
#
# Only the aws_iam_role data source is overridden (not the whole provider): the
# aws_iam_policy_document data sources render their .json client-side, so leaving
# them un-mocked exercises the real least-privilege policies in this mode too.

variables {
  name               = "demo"
  mode               = "byo"
  region             = "us-east-1"
  account_id         = "111122223333"
  oidc_provider_arn  = "arn:aws:iam::111122223333:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"
  oidc_issuer_url    = "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"
  namespace          = "workload-system"
  service_account    = "workload"
  kms_key_arn        = "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
  secret_path_prefix = "demo"
  ecr_repo_arns      = ["arn:aws:ecr:us-east-1:111122223333:repository/workload"]
  provided_role_arn  = "arn:aws:iam::111122223333:role/customer-byo-workload"
}

run "byo_resolves_role_and_emits_docs" {
  command = plan

  override_data {
    target = data.aws_iam_role.byo[0]
    values = {
      arn = "arn:aws:iam::111122223333:role/customer-byo-workload"
    }
  }

  assert {
    condition     = length(aws_iam_role.workload) == 0
    error_message = "byo-identity mode must NOT create an IAM role."
  }
  assert {
    condition     = length(aws_iam_role_policy.runtime) == 0
    error_message = "byo-identity mode must NOT attach an inline role policy (the customer attaches the emitted docs)."
  }
  assert {
    condition     = length(data.aws_iam_role.byo) == 1
    error_message = "byo-identity mode must look up exactly the supplied role."
  }
  assert {
    condition     = output.role_arn == "arn:aws:iam::111122223333:role/customer-byo-workload"
    error_message = "byo-identity mode must expose the resolved role ARN under the same output key as provision mode."
  }
  # The reviewable artifacts must still render so the customer can attach them.
  assert {
    condition     = output.runtime_policy_json != "" && output.trust_policy_json != "" && output.deploy_policy_json != ""
    error_message = "byo-identity mode must still render the runtime, trust, and deploy policy documents for the customer to attach."
  }
  # The emitted runtime doc is the same least-privilege policy as provision mode:
  # scoped to the resolved key ARN and the secret path prefix.
  assert {
    condition     = strcontains(output.runtime_policy_json, var.kms_key_arn) && strcontains(output.runtime_policy_json, "secret:${var.secret_path_prefix}-*")
    error_message = "byo-identity emitted runtime policy must carry the same key/secret scoping as provision mode."
  }
}
