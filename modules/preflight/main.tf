# Invoke the preflight checker binary. The hashicorp `external` data source requires a flat
# string->string JSON object on stdout; the binary prints {"verdict":...,"report_json":...} and
# exits 0, so the data source never errors and the gate keys on the parsed verdict.
#
# The hard gate is the postcondition below. The data source is read at plan time, before any
# resource is created, and every downstream value derives from its result — so a red verdict fails
# the plan on the critical path. result is plan-time-known, so install_tier can drive `count`.
data "external" "preflight" {
  program = [
    var.preflight_binary,
    "--mode=agnostic",
    "--kubeconfig", var.kubeconfig_path,
    "--namespace", var.namespace,
  ]

  lifecycle {
    postcondition {
      # Hard block: a red verdict fails the plan unless fail_on_red is disabled.
      # `self` avoids a locals<->data-source reference cycle.
      condition     = !(var.fail_on_red && self.result.verdict == "red")
      error_message = "Preflight verdict is RED — deployment blocked. Report: ${self.result.report_json}"
    }
  }
}

locals {
  # Top-level gate value: "green" | "amber" | "red".
  verdict = data.external.preflight.result.verdict

  # The full Report, recovered by decoding the double-encoded report_json string.
  report = jsondecode(data.external.preflight.result.report_json)

  # Find the Stage 4 install-tier result (stage id == 4, result id == k8s.installtier). Use a
  # length-guarded index rather than one(), so a malformed report falls through to the floor below
  # instead of crashing the plan.
  stage4_list = [for s in local.report.stages : s if s.id == 4]
  stage4      = length(local.stage4_list) > 0 ? local.stage4_list[0] : null

  installtier_list = local.stage4 == null ? [] : [
    for r in local.stage4.results : r if r.id == "k8s.installtier"
  ]
  installtier_status = length(local.installtier_list) > 0 ? local.installtier_list[0].status : ""

  # Derive tier from the install-tier check status:
  #   green   -> "A" (operator; cluster-scoped CRD+ClusterRole creatable)
  #   amber   -> "B" (namespace-only manifests)
  #   red     -> "RED", an invalid tier that trips the downstream module validation rather than
  #              attempting a deploy the identity cannot perform (reached only if fail_on_red=false).
  #   missing -> "B" (safe namespaced floor)
  derived_install_tier = (
    local.installtier_status == "green" ? "A" :
    local.installtier_status == "amber" ? "B" :
    local.installtier_status == "red" ? "RED" : "B"
  )

  # Explicit override wins when set.
  install_tier = var.install_tier_override != "" ? var.install_tier_override : local.derived_install_tier
}

# Surface an amber verdict as a non-blocking plan warning with the full report. Red is handled by
# the postcondition above; it is not re-asserted here.
check "preflight_amber" {
  assert {
    condition     = local.verdict != "amber"
    error_message = "Preflight AMBER (proceeding with documented gaps): ${data.external.preflight.result.report_json}"
  }
}
