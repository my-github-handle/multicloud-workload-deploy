# Plan-time assertions that the rendered IAM policies are wildcard-free and
# resource-scoped (the headline least-privilege claim). Runs offline
# (command = plan): the aws_iam_policy_document data sources render their .json at
# plan time, so no AWS account is needed.

variables {
  name               = "demo"
  mode               = "provision"
  region             = "us-east-1"
  account_id         = "111122223333"
  oidc_provider_arn  = "arn:aws:iam::111122223333:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"
  oidc_issuer_url    = "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"
  namespace          = "workload-system"
  service_account    = "workload"
  kms_key_arn        = "arn:aws:kms:us-east-1:111122223333:key/abcd-1234"
  secret_path_prefix = "demo"
  ecr_repo_arns      = ["arn:aws:ecr:us-east-1:111122223333:repository/workload"]
}

run "runtime_policy_has_no_forbidden_wildcards" {
  command = plan

  assert {
    condition     = length(regexall("kms:\\*", data.aws_iam_policy_document.runtime.json)) == 0
    error_message = "runtime policy must not contain kms:* — scope to the resolved key ARN."
  }
  assert {
    condition     = length(regexall("secretsmanager:\\*", data.aws_iam_policy_document.runtime.json)) == 0
    error_message = "runtime policy must not contain secretsmanager:* — scope to the secret path prefix."
  }
  # Resource:"*" is allowed EXACTLY ONCE — the non-scopable ecr:GetAuthorizationToken
  # statement. More than one means a real wildcard leak.
  assert {
    condition     = length(regexall("\"Resource\": \"\\*\"", data.aws_iam_policy_document.runtime.json)) <= 1
    error_message = "runtime policy has more than the single allowed Resource:* (ecr:GetAuthorizationToken)."
  }
  assert {
    condition     = strcontains(data.aws_iam_policy_document.runtime.json, var.kms_key_arn)
    error_message = "runtime policy must scope KMS actions to the resolved key ARN."
  }
  assert {
    condition     = strcontains(data.aws_iam_policy_document.runtime.json, "secret:${var.secret_path_prefix}-*")
    error_message = "runtime policy must scope GetSecretValue to the secret path-prefix, not a bare wildcard."
  }
  # No-drift: the runtime policy must contain EXACTLY the expected action set.
  # Adding/removing a runtime action without updating this list fails the test.
  assert {
    condition = alltrue([
      for a in [
        "kms:Decrypt", "kms:GenerateDataKey", "kms:DescribeKey",
        "secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret",
        "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage",
        "ecr:BatchCheckLayerAvailability", "ecr:GetAuthorizationToken",
      ] : strcontains(data.aws_iam_policy_document.runtime.json, a)
    ])
    error_message = "runtime policy action set drifted from the expected snapshot — update this assertion AND review the change."
  }
}

run "deploy_policy_is_region_account_scoped" {
  command = plan

  # The deploy policy's only Resource:"*" is the CreateKey statement (no
  # pre-existing ARN; constrained by the aws:RequestedRegion condition); every
  # other statement is ARN-pinned.
  assert {
    condition     = length(regexall("\"Resource\": \"\\*\"", data.aws_iam_policy_document.deploy.json)) <= 1
    error_message = "deploy policy must not use more than the single region-conditioned Resource:* (KMS CreateKey)."
  }
  assert {
    condition     = strcontains(data.aws_iam_policy_document.deploy.json, var.region) && strcontains(data.aws_iam_policy_document.deploy.json, var.account_id)
    error_message = "deploy policy must pin BOTH region and account."
  }
  # No-drift: the deploy policy must contain EXACTLY the expected keystone action
  # set. This is also the anchor for the Go preflight requiredDeployActions probe,
  # which must stay a subset of this list, so probe and artifact cannot diverge.
  assert {
    condition = alltrue([
      for a in [
        "kms:CreateKey", "kms:CreateAlias", "kms:EnableKeyRotation",
        "secretsmanager:CreateSecret", "secretsmanager:PutSecretValue",
        "iam:CreateRole", "iam:PutRolePolicy", "iam:AttachRolePolicy",
        "eks:CreateCluster", "eks:DescribeCluster",
      ] : strcontains(data.aws_iam_policy_document.deploy.json, a)
    ])
    error_message = "deploy policy action set drifted from the expected snapshot — update this assertion, keep the Go requiredDeployActions probe a subset, AND review the change."
  }
}
