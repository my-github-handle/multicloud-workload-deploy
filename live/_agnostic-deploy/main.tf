locals {
  # Decode the workload spec once: the workload module takes the raw YAML, the security module takes
  # the port derived here, so the two share one source. A missing/non-numeric port fails the plan.
  workload_spec = yamldecode(var.workload_spec_yaml)
  workload_port = local.workload_spec.port
}

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
  # For the CRD-Established wait (Tier A): same kubeconfig/context as the providers.
  kubeconfig_path = var.kubeconfig_path
  kube_context    = var.kube_context
}

# Stage 2: Security. manage_namespace is true only in Tier B (the operator chart creates the
# namespace in Tier A); either way the namespace ends up labelled exactly once. workload_selector_
# labels keeps its default ({} = namespace-wide) so the policies cannot drift from the chart's pod
# labels. workload_port comes from the decoded spec so the allow policy matches the serving port.
module "k8s_security" {
  source = "../../modules/k8s-security"

  namespace          = var.namespace
  manage_namespace   = module.preflight.install_tier == "B"
  control_plane_port = var.control_plane_port
  psa_enforce_level  = var.psa_enforce_level
  workload_port      = local.workload_port

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

  # Order after k8s_platform (operator + CRD-Established wait, so the Workload kind is registered
  # before the CR applies) and k8s_security (namespace labelled before the CR's pods are admitted).
  depends_on = [module.k8s_platform, module.k8s_security]
}
