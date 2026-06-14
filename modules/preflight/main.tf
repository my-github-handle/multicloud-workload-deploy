# Invoke the preflight checker binary. The hashicorp `external` data source requires the program
# to print a FLAT JSON object of string->string on stdout; the binary prints exactly
# {"verdict":"...","report_json":"..."} and always exits 0 (no --exit-on-red), so this data source
# never errors and we can gate on the parsed verdict instead.
#
# THE HARD GATE LIVES HERE, on the data source's postcondition — NOT on a separate terraform_data
# resource. This is deliberate and load-bearing:
#   - An `external` data source is read during `terraform plan`, BEFORE any resource is created. A
#     failing postcondition halts the plan, so a red verdict blocks deployment before any resource
#     is created.
#   - EVERY downstream consumer reads `data.external.preflight.result` (verdict, report,
#     install_tier all derive from it), so the check is unavoidably on the critical path of the
#     whole graph.
#   - `result` is known at plan time, so `install_tier` stays plan-time-known and can legally drive
#     `count` in k8s-platform/workload.
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

  # Find the Stage 4 (kubernetes infra) install-tier check result. report.stages is a list; the
  # stage with id == 4 holds results[] with the k8s.installtier entry. We use a filtered list +
  # [0] guarded by length(), NOT one(): one() throws on a list of length != 1, so a
  # malformed/duplicated report would convert a data anomaly into a hard plan crash. Here a missing
  # entry falls through to the safe floor below.
  stage4_list = [for s in local.report.stages : s if s.id == 4]
  stage4      = length(local.stage4_list) > 0 ? local.stage4_list[0] : null

  installtier_list = local.stage4 == null ? [] : [
    for r in local.stage4.results : r if r.id == "k8s.installtier"
  ]
  installtier_status = length(local.installtier_list) > 0 ? local.installtier_list[0].status : ""

  # Derive tier from the install-tier check status:
  #   green   -> "A" (operator; cluster-scoped CRD+ClusterRole creatable)
  #   amber   -> "B" (namespace-only; operator-less namespaced manifests)
  #   red     -> the identity cannot create even namespaced workloads. The overall verdict is then
  #              red in this case, so the gate above blocks the apply. We surface "RED" here (an
  #              invalid tier) rather than silently deriving "B": if fail_on_red is disabled,
  #              deriving "B" would attempt a namespaced deploy the identity provably cannot
  #              perform. An invalid tier trips k8s-platform's install_tier validation, failing
  #              loudly instead of half-deploying.
  #   missing -> "B" as the safe namespaced floor (e.g. k8s stages not run).
  derived_install_tier = (
    local.installtier_status == "green" ? "A" :
    local.installtier_status == "amber" ? "B" :
    local.installtier_status == "red" ? "RED" : "B"
  )

  # Explicit override wins when set.
  install_tier = var.install_tier_override != "" ? var.install_tier_override : local.derived_install_tier
}

# Rich, always-on reporting that native gating cannot express on its own. This check block asserts
# ONLY the amber/non-blocking case. The RED hard gate lives exclusively in the data source's
# postcondition above — duplicating a `verdict != "red"` assert here would emit the red failure
# twice (once as the postcondition error, once as a check-block warning). So: red is blocked by the
# postcondition; amber is surfaced here as a non-blocking `terraform plan` warning with the report.
check "preflight_amber" {
  assert {
    condition     = local.verdict != "amber"
    error_message = "Preflight AMBER (proceeding with documented gaps): ${data.external.preflight.result.report_json}"
  }
}
