locals {
  is_tier_a = var.install_tier == "A"
}

# Tier A: install charts/workload-operator (CRD via the chart's crds/ dir, namespace-scoped
# Role/RoleBinding, the controller Deployment, ServiceAccount). The chart installs CRDs ahead of
# templates. The controller runs namespace-scoped via watchNamespace.
#
# Tier B: count = 0 — no operator, no CRD. The workload module renders charts/workload directly.
resource "helm_release" "operator" {
  count = local.is_tier_a ? 1 : 0

  name             = "workload-operator"
  namespace        = var.namespace
  create_namespace = var.create_namespace
  chart            = "${path.module}/${var.operator_chart_path}"

  # Wait for the controller Deployment to become ready so downstream modules (and the Tier A
  # Workload CR apply) see a live operator + registered CRD.
  wait    = true
  timeout = 300

  set {
    name  = "namespace"
    value = var.namespace
  }
  set {
    name  = "image.repository"
    value = var.operator_image_repository
  }
  set {
    name  = "image.tag"
    value = var.operator_image_tag
  }
  set {
    name  = "watchNamespace"
    value = var.namespace
  }
}
