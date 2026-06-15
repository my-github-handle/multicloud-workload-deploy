locals {
  is_byo = var.mode == "byo"
}

# BYO: look up the existing cluster's endpoint + CA.
data "aws_eks_cluster" "byo" {
  count = local.is_byo ? 1 : 0
  name  = var.cluster_name
}

locals {
  # `endpoint` is the FULL https:// endpoint. EKS endpoints are already https://;
  # if a bare host is ever supplied (a BYO edge case) it is normalized to include
  # the scheme here, inside the resolver, so every consumer receives a uniform
  # full-URL endpoint.
  raw_endpoint = local.is_byo ? data.aws_eks_cluster.byo[0].endpoint : var.provisioned_endpoint
  resolved_endpoint = (
    local.raw_endpoint == "" ? "" :
    startswith(local.raw_endpoint, "https://") ? local.raw_endpoint : "https://${local.raw_endpoint}"
  )
  resolved_ca = local.is_byo ? data.aws_eks_cluster.byo[0].certificate_authority[0].data : var.provisioned_ca

  # auth is a tagged object, not a bare token. The default form is the EKS
  # exec-plugin (aws eks get-token), which the kubernetes/helm/kubectl providers
  # invoke at apply time to fetch a FRESH token from the provider process. This
  # avoids the data.aws_eks_cluster_auth token-churn problem (that data source
  # re-reads a short-lived token on every plan, producing a perpetual diff and an
  # expiring token baked into state).
  #
  # auth.kind discriminates the shape; consumers switch on it. AWS uses "exec".
  # The "token" variant is kept for clouds/edge cases that supply a static bearer
  # token, matching the cross-cloud {endpoint, ca, auth} interface.
  resolved_auth = {
    kind = "exec"
    exec = {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", var.cluster_name, "--output", "json"]
    }
    # token is left null in the exec form; populated only when kind == "token".
    token = null
  }
}
