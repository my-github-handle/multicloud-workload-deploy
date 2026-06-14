locals {
  is_tier_a = var.install_tier == "A"
  is_tier_b = var.install_tier == "B"

  # The single source of the workload's shape. Decoded once; both tiers derive from it so the CR
  # spec and the chart values cannot drift.
  spec = yamldecode(var.spec_yaml)

  # Tier A: the Workload CR. spec is the decoded YAML verbatim — every field the CRD accepts flows
  # through untouched (image, port, autoscale, probes, resources, securityContext,
  # podSecurityContext, rolloutStrategy, ingress, ...).
  workload_manifest = {
    apiVersion = "workload.ops.dev/v1"
    kind       = "Workload"
    metadata = {
      name      = var.name
      namespace = var.namespace
    }
    spec = local.spec
  }

  # Tier B: charts/workload values. The same decoded spec, plus the identity (name/namespace) and
  # the chart-only PDB knob the CRD does not expose. helm_release's `values` takes a YAML document,
  # so nested fields (autoscale, probes, security contexts, resources) pass through without the
  # per-key `set`/tostring coercion the old typed-variable form required.
  helm_values = merge(
    local.spec,
    {
      name      = var.name
      namespace = var.namespace
      pdb       = { minAvailable = var.pdb_min_available }
    },
  )
}

# Tier A: apply the Workload custom resource; the operator (installed by k8s-platform) reconciles
# it into Deployment/Service/HPA/PDB and owns lifecycle, status conditions, and drift correction.
#
# Uses kubectl_manifest (alekc/kubectl), NOT kubernetes_manifest: kubernetes_manifest performs
# CRD/OpenAPI schema discovery against a live cluster at plan time, so it cannot plan the CR before
# the operator chart installs the CRD — a chicken-and-egg that breaks single-apply on a fresh
# cluster. kubectl_manifest takes raw YAML and defers schema handling to apply time. Apply ordering
# is wired at the root via module.workload's depends_on on module.k8s_platform.
resource "kubectl_manifest" "workload_cr" {
  count = local.is_tier_a ? 1 : 0

  yaml_body         = yamlencode(local.workload_manifest)
  server_side_apply = true

  # Block on the operator-set Ready condition so this resource is not "done" until the Workload is
  # actually reconciled healthy. Gated + time-bounded: on a rate-limited/slow cluster set
  # wait_for_ready=false and confirm readiness out-of-band (kubectl wait), so a throttled api-server
  # poll does not fail an otherwise-successful apply.
  wait = var.wait_for_ready
  timeouts {
    create = var.wait_timeout
  }
  dynamic "wait_for" {
    for_each = var.wait_for_ready ? [1] : []
    content {
      field {
        key   = "status.conditions.[type=Ready].status"
        value = "True"
      }
    }
  }
}

# Tier B: no operator/CRD available — render the same charts/workload chart directly into the
# namespace (Deployment/Service/HPA/PDB), with lifecycle driven by terraform apply. The values are
# the same decoded spec, so the manifests are identical to what the operator renders.
resource "helm_release" "workload" {
  count = local.is_tier_b ? 1 : 0

  name      = var.name
  namespace = var.namespace
  chart     = "${path.module}/${var.workload_chart_path}"
  wait      = true
  timeout   = 300

  values = [yamlencode(local.helm_values)]
}
