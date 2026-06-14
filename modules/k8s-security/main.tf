# Namespaced PodSecurity: label the workload namespace so admission rejects non-conformant pods.
# Pods in this namespace (workload, connect-agent, and in Tier A the operator) must satisfy the
# enforce level. Two paths, both ending with the namespace labelled exactly once:
#   - manage_namespace = true:  create the namespace with the labels.
#   - manage_namespace = false: label the existing (operator-created) namespace via kubernetes_labels.
locals {
  # enforce is configurable (default restricted); audit/warn always track restricted so the gap
  # from the secure floor stays visible even when enforce is relaxed to baseline.
  psa_labels = {
    "pod-security.kubernetes.io/enforce" = var.psa_enforce_level
    "pod-security.kubernetes.io/audit"   = "restricted"
    "pod-security.kubernetes.io/warn"    = "restricted"
  }
}

resource "kubernetes_namespace" "this" {
  count = var.manage_namespace ? 1 : 0

  metadata {
    name   = var.namespace
    labels = local.psa_labels
  }
}

# Tier A: label the operator-created namespace in place. force avoids fighting the operator chart
# over ownership of these specific label keys (it manages only the PSA keys, not the whole object).
resource "kubernetes_labels" "psa" {
  count = var.manage_namespace ? 0 : 1

  api_version = "v1"
  kind        = "Namespace"
  metadata {
    name = var.namespace
  }
  labels = local.psa_labels
  force  = true
}

# Default-deny ALL ingress and egress for workload pods. Nothing flows unless an explicit allow
# policy (below) re-opens it. This blocks pod-to-pod, the control plane, and crucially the cloud
# metadata endpoint by default.
resource "kubernetes_network_policy" "default_deny" {
  metadata {
    name      = "default-deny-all"
    namespace = var.namespace
  }
  spec {
    pod_selector {
      match_labels = var.workload_selector_labels
    }
    policy_types = ["Ingress", "Egress"]
    # No ingress{} and no egress{} blocks => deny all in both directions.
  }
}

# Explicit allowlist layered on the namespace-wide default-deny.
#   Egress:  DNS (53); the control-plane port on 0.0.0.0/0 with the cloud metadata IPs
#            (169.254.169.254/32 and 169.254.0.0/16) carved out via except so the credential-theft
#            endpoint is unreachable; and intra-namespace traffic to the workload port.
#   Ingress: intra-namespace traffic to the workload port.
# FQDN-granular egress is not attempted here (plain NetworkPolicy is CIDR/port-based). Set
# workload_port = 0 to omit the workload-port allowances.
resource "kubernetes_network_policy" "allow" {
  metadata {
    name      = "allow-dns-controlplane-and-workload"
    namespace = var.namespace
  }
  spec {
    pod_selector {
      match_labels = var.workload_selector_labels
    }
    policy_types = ["Ingress", "Egress"]

    # Intra-namespace ingress to the workload serving port (omitted when workload_port == 0).
    dynamic "ingress" {
      for_each = var.workload_port > 0 ? [1] : []
      content {
        from {
          namespace_selector {
            match_labels = {
              "kubernetes.io/metadata.name" = var.namespace
            }
          }
        }
        ports {
          protocol = "TCP"
          port     = tostring(var.workload_port)
        }
      }
    }

    # Allow DNS to the cluster DNS namespace (UDP+TCP 53).
    egress {
      to {
        namespace_selector {
          match_labels = {
            "kubernetes.io/metadata.name" = var.dns_namespace
          }
        }
      }
      ports {
        protocol = "UDP"
        port     = "53"
      }
      ports {
        protocol = "TCP"
        port     = "53"
      }
    }

    # Intra-namespace egress to the workload serving port (omitted when workload_port == 0).
    dynamic "egress" {
      for_each = var.workload_port > 0 ? [1] : []
      content {
        to {
          namespace_selector {
            match_labels = {
              "kubernetes.io/metadata.name" = var.namespace
            }
          }
        }
        ports {
          protocol = "TCP"
          port     = tostring(var.workload_port)
        }
      }
    }

    # Egress to the control-plane port on all destinations except the cloud metadata IPs.
    egress {
      to {
        ip_block {
          cidr   = "0.0.0.0/0"
          except = ["169.254.169.254/32", "169.254.0.0/16"]
        }
      }
      ports {
        protocol = "TCP"
        port     = tostring(var.control_plane_port)
      }
    }
  }
}
