locals {
  # Strip the scheme from the issuer URL for the IRSA sub/aud condition keys.
  oidc_host = replace(var.oidc_issuer_url, "https://", "")
}

# --- IRSA trust policy: only the workload ServiceAccount in the workload
#     namespace, federated through the cluster's OIDC provider, may assume this
#     role (a resource-scoped principal). ---
data "aws_iam_policy_document" "trust" {
  statement {
    sid     = "IRSATrust"
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [var.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:sub"
      values   = ["system:serviceaccount:${var.namespace}:${var.service_account}"]
    }
    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

# --- Runtime workload identity: the minimum the workload + connect-agent need.
#     kms:Decrypt/GenerateDataKey on the resolved key only; GetSecretValue on the
#     secret path prefix only; ECR pull on the named repos only. No wildcards;
#     region pinned. ---
data "aws_iam_policy_document" "runtime" {
  statement {
    sid    = "DecryptWithResolvedCMK"
    effect = "Allow"
    actions = [
      "kms:Decrypt",
      "kms:GenerateDataKey",
      "kms:DescribeKey",
    ]
    resources = [var.kms_key_arn]

    condition {
      test     = "StringEquals"
      variable = "aws:RequestedRegion"
      values   = [var.region]
    }
  }

  # Scoped to the Secrets Manager path prefix, not per-secret-arn. AWS appends a
  # random 6-char suffix to each secret name, so `<prefix>-*` matches every secret
  # the secrets module creates under this prefix — and keeps iam independent of
  # secrets (no module dependency on secrets.secret_arns).
  statement {
    sid    = "ReadResolvedSecrets"
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret",
    ]
    resources = ["arn:aws:secretsmanager:${var.region}:${var.account_id}:secret:${var.secret_path_prefix}-*"]
  }

  # Only emit the ECR pull statement when repos are supplied (no empty Resource list).
  dynamic "statement" {
    for_each = length(var.ecr_repo_arns) > 0 ? [1] : []
    content {
      sid    = "PullWorkloadImages"
      effect = "Allow"
      actions = [
        "ecr:GetDownloadUrlForLayer",
        "ecr:BatchGetImage",
        "ecr:BatchCheckLayerAvailability",
      ]
      resources = var.ecr_repo_arns
    }
  }

  # ecr:GetAuthorizationToken cannot be resource-scoped by AWS design — it is a
  # read-only token vendor that grants no data access. The single Resource:"*" in
  # the runtime policy is here and is intentional.
  statement {
    sid       = "ECRAuthToken"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
}

# --- Deploy-time identity: create/manage only the resources in the aws-full path,
#     scoped to this account+region. Used by the terraform apply operator. ---
data "aws_iam_policy_document" "deploy" {
  # CreateKey has no pre-existing ARN to scope to, so it is constrained by the
  # region condition rather than a resource ARN.
  statement {
    sid    = "ManageWorkloadKMS"
    effect = "Allow"
    actions = [
      "kms:CreateKey",
      "kms:CreateAlias",
      "kms:DescribeKey",
      "kms:EnableKeyRotation",
      "kms:TagResource",
    ]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "aws:RequestedRegion"
      values   = [var.region]
    }
  }

  statement {
    sid    = "ManageWorkloadSecrets"
    effect = "Allow"
    actions = [
      "secretsmanager:CreateSecret",
      "secretsmanager:PutSecretValue",
      "secretsmanager:DescribeSecret",
      "secretsmanager:TagResource",
    ]
    resources = ["arn:aws:secretsmanager:${var.region}:${var.account_id}:secret:${var.name}-*"]
  }

  statement {
    sid    = "ManageWorkloadIAMRole"
    effect = "Allow"
    actions = [
      "iam:CreateRole",
      "iam:GetRole",
      "iam:PutRolePolicy",
      "iam:AttachRolePolicy",
      "iam:TagRole",
    ]
    resources = ["arn:aws:iam::${var.account_id}:role/${var.name}-*"]
  }

  # EKS cluster create/manage for greenfield provisioning. eks:CreateCluster is the
  # keystone action the Go Stage-0 preflight probe simulates, so it must appear here
  # to keep the probe a subset of the rendered deploy artifact.
  statement {
    sid    = "ManageWorkloadEKS"
    effect = "Allow"
    actions = [
      "eks:CreateCluster",
      "eks:DescribeCluster",
      "eks:TagResource",
    ]
    resources = ["arn:aws:eks:${var.region}:${var.account_id}:cluster/${var.name}*"]
  }
}
