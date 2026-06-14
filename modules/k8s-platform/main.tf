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

# Tier A only: wait for the Workload CRD to reach its Established condition after the operator
# install, so a same-apply Workload CR is not rejected before the kind is registered. Exposed via
# the crd_established output for the workload module to depend on. Uses kubectl + the kubeconfig.
resource "terraform_data" "crd_established" {
  count = local.is_tier_a ? 1 : 0

  # Re-run when the operator release changes.
  triggers_replace = [helm_release.operator[0].id]

  provisioner "local-exec" {
    command = "kubectl --kubeconfig=${var.kubeconfig_path} ${var.kube_context != "" ? "--context=${var.kube_context}" : ""} wait --for=condition=established crd/${var.crd_name} --timeout=${var.crd_wait_timeout}"
  }
}
