# Namespaced PodSecurity (restricted): enforce via namespace labels so the admission controller
# rejects non-conformant pods.
#
# In Tier A the operator pod (and, when enabled, the connect-agent pod) run in this same namespace,
# which we label enforce=restricted. PSA admission rejects those pods at creation unless their pod
# templates are restricted-compliant: runAsNonRoot=true, seccompProfile.type=RuntimeDefault,
# capabilities.drop=["ALL"], allowPrivilegeEscalation=false, no host namespaces, no privileged. The
# operator and connect-agent pod templates must be restricted-compliant — verified by the kind run
# (a non-compliant template fails to schedule, failing the apply loudly).
#
# Two paths, both ending with the namespace labelled `restricted` exactly once:
#   - manage_namespace = true  (Tier B): create the namespace WITH the labels (no unlabeled window).
#   - manage_namespace = false (Tier A): the operator chart already created the namespace, so attach
#     the labels to the existing namespace via kubernetes_labels. Without this, a Tier A install
#     would run with NO PodSecurity enforcement (the operator chart does not set PSA labels).
locals {
  # enforce is configurable (default restricted); audit/warn always track restricted so the gap
  # from the secure floor stays visible in events/warnings even when enforce is relaxed to baseline.
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

# Explicit allowlist layered on the namespace-wide default-deny. Egress: DNS (name resolution) + a
# wide CIDR on the control-plane PORT (NOT an FQDN — plain NetworkPolicy cannot match FQDNs; FQDN
# scoping is the perimeter firewall / Cilium toFQDNs) + intra-namespace traffic to the workload
# port. Ingress: intra-namespace traffic to the workload port. The cloud metadata IP
# 169.254.169.254 is a /32 carved out of the allowed CIDR via except (plus the wider
# 169.254.0.0/16 link-local range), so even the broad egress allow can never reach it — a primary
# credential-theft vector is blocked at the CNI layer regardless of the default-deny.
#
# The workload-port allowances are what keep this namespace-wide floor from strangling the
# workload's own traffic: without them, the empty-selector default-deny would block intra-namespace
# connections to the workload's serving port even though charts/workload's per-workload allow
# policy permits them. Set workload_port = 0 to omit these.
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

    # Allow egress to the control-plane port on all non-metadata destinations. 0.0.0.0/0 minus the
    # metadata /32 — the connect-agent FQDN resolves to a public IP inside this range, while
    # 169.254.169.254 is explicitly excluded.
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
