# Ship the rendered policy documents as reviewable artifacts so a customer can
# inspect them (and, in byo-identity mode, attach them) before granting anything.
# Both the deploy-time and runtime policies are rendered; the golden test asserts
# no-wildcards + resource-pinning on both.
resource "local_file" "deploy_policy" {
  filename        = "${path.module}/${var.artifacts_dir}/deploy-policy.json"
  content         = data.aws_iam_policy_document.deploy.json
  file_permission = "0644"
}

resource "local_file" "runtime_policy" {
  filename        = "${path.module}/${var.artifacts_dir}/runtime-policy.json"
  content         = data.aws_iam_policy_document.runtime.json
  file_permission = "0644"
}

resource "local_file" "trust_policy" {
  filename        = "${path.module}/${var.artifacts_dir}/trust-policy.json"
  content         = data.aws_iam_policy_document.trust.json
  file_permission = "0644"
}

# The live runtime policy is path-prefix scoped (no per-arn module edge). For
# reviewer visibility, the concrete secret ARNs (when supplied via
# recorded_secret_arns) are recorded in a companion artifact. Documentation only —
# it does not widen or narrow the granted policy.
resource "local_file" "recorded_secret_arns" {
  count    = length(var.recorded_secret_arns) > 0 ? 1 : 0
  filename = "${path.module}/${var.artifacts_dir}/runtime-secret-arns.json"
  content = jsonencode({
    note               = "Concrete secret ARNs covered by the path-prefix-scoped runtime policy (review only)."
    secret_path_prefix = var.secret_path_prefix
    secret_arns        = var.recorded_secret_arns
  })
  file_permission = "0644"
}
