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

  # Tier B: charts/workload values — the same decoded spec, plus the identity (name/namespace) and
  # the chart-only PDB knob the CRD does not expose. Passed to helm_release as a YAML document, so
  # nested fields (autoscale, probes, security contexts, resources) pass through as-is.
  helm_values = merge(
    local.spec,
    {
      name      = var.name
      namespace = var.namespace
      pdb       = { minAvailable = var.pdb_min_available }
    },
  )
}

# Ordering gate: holds the upstream readiness handles (the Workload CRD being
# Established, the secret material existing). The workload resources reference it
# via replace_triggered_by so they apply only after those upstreams exist.
resource "terraform_data" "ordering" {
  input = "${var.crd_ready}:${var.secrets_ready}"
}

# Tier A: apply the Workload custom resource; the operator reconciles it into
# Deployment/Service/HPA/PDB and owns its lifecycle, status, and drift correction.
#
# kubectl_manifest applies raw YAML with no plan-time CRD schema discovery, so the CR can be planned
# before its CRD exists and applied in the same run after the operator installs it. The
# replace_triggered_by reference to terraform_data.ordering gates the apply on the upstreams.
resource "kubectl_manifest" "workload_cr" {
  count = local.is_tier_a ? 1 : 0

  yaml_body         = yamlencode(local.workload_manifest)
  server_side_apply = true

  lifecycle {
    replace_triggered_by = [terraform_data.ordering]
  }

  wait = var.wait_for_ready
  timeouts {
    create = var.wait_timeout
  }
  # Wait for the operator-set Ready status condition. condition{} matches a named status condition;
  # the field-path matcher does not support condition list-filtering.
  dynamic "wait_for" {
    for_each = var.wait_for_ready ? [1] : []
    content {
      condition {
        type   = "Ready"
        status = "True"
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
