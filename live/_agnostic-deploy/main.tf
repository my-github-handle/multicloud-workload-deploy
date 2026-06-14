# Stage 0: Preflight. Invokes the checker binary, gates on a red verdict, and derives the install
# tier. Everything downstream depends on the derived tier, so the gate evaluates before any
# resource is created.
module "preflight" {
  source = "../../modules/preflight"

  preflight_binary      = var.preflight_binary
  kubeconfig_path       = var.kubeconfig_path
  namespace             = var.namespace
  fail_on_red           = var.fail_on_red
  install_tier_override = var.install_tier_override
}

# Stage 1: Platform. Tier A installs the operator chart (creates ns, CRD, namespace-scoped RBAC,
# controller); Tier B is a no-op (count=0).
module "k8s_platform" {
  source = "../../modules/k8s-platform"

  install_tier              = module.preflight.install_tier
  namespace                 = var.namespace
  operator_image_repository = var.operator_image_repository
  operator_image_tag        = var.operator_image_tag
  create_namespace          = true
}

# Stage 2: Security. In Tier B the operator chart did NOT create the namespace, so this module
# creates+labels it. In Tier A the operator chart created the namespace, so this module does NOT
# re-create it but STILL applies the restricted PodSecurity labels to the existing namespace.
# Either way the namespace ends up labelled restricted exactly once.
#
# workload_selector_labels is left at its default ({} = namespace-wide default-deny). Do NOT
# override it with a managed-by selector: charts/workload pods carry only app.kubernetes.io/name,
# so a managed-by selector matches no pods and the policies become inert.
module "k8s_security" {
  source = "../../modules/k8s-security"

  namespace          = var.namespace
  manage_namespace   = module.preflight.install_tier == "B"
  control_plane_port = var.control_plane_port
  psa_enforce_level  = var.psa_enforce_level
  # Derive the workload port from the single spec_yaml source so the namespace allow policy permits
  # the workload's own serving port and cannot drift from the workload's container port.
  workload_port = try(tonumber(yamldecode(var.workload_spec_yaml).port), 8080)

  depends_on = [module.k8s_platform]
}

# Stage 3: Observability (in-cluster only). Cloud VPC flow logs are out of scope here — they live
# in the per-cloud network module.
module "k8s_observability" {
  source = "../../modules/k8s-observability"

  namespace = var.namespace
  enabled   = var.observability_enabled

  depends_on = [module.k8s_platform]
}

# Stage 4: Workload. Tier A applies the Workload CR (operator reconciles); Tier B renders
# charts/workload directly. Same inputs both tiers.
module "workload" {
  source = "../../modules/workload"

  install_tier   = module.preflight.install_tier
  name           = var.workload_name
  namespace      = var.namespace
  spec_yaml      = var.workload_spec_yaml
  wait_for_ready = var.workload_wait_for_ready

  # crd_ready threads the operator release name into the workload module purely as a documented
  # ordering handle (it is NOT used in a depends_on inside the module). Empty string in Tier B.
  crd_ready = module.k8s_platform.operator_release_name

  # Apply ordering is enforced here, at the root: module.workload depends on k8s_platform (operator
  # chart installs the CRD + creates the namespace in Tier A) and k8s_security (labels the namespace
  # restricted). This module-level depends_on orders the Tier A Workload CR after the operator/CRD
  # within the single apply.
  #
  # CRD-Established race: helm_release wait=true on the operator chart waits only for the chart's
  # resources, NOT for the api-server to report the Workload CRD's Established condition. The kind
  # runbook inserts an explicit
  #   kubectl wait --for=condition=established crd/workloads.workload.ops.dev
  # gate between the operator install and the CR apply to close this race.
  depends_on = [module.k8s_platform, module.k8s_security]
}
