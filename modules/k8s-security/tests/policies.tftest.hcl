# Plan-time assertions on the NetworkPolicy shape. No cluster contacted (command = plan). Run with
# `terraform test` from modules/k8s-security.

variables {
  namespace          = "demo-ns"
  control_plane_port = 443
  workload_port      = 8080
}

run "policies_enforce_the_security_floor" {
  command = plan

  # 1. default-deny selects all pods and opens NOTHING: no ingress, no egress blocks, both policy
  #    types present => deny-all in both directions.
  assert {
    condition     = length(kubernetes_network_policy.default_deny.spec[0].ingress) == 0
    error_message = "default-deny must have NO ingress rules (empty => deny all ingress)."
  }
  assert {
    condition     = length(kubernetes_network_policy.default_deny.spec[0].egress) == 0
    error_message = "default-deny must have NO egress rules (empty => deny all egress)."
  }
  assert {
    condition = (
      contains(kubernetes_network_policy.default_deny.spec[0].policy_types, "Ingress") &&
      contains(kubernetes_network_policy.default_deny.spec[0].policy_types, "Egress")
    )
    error_message = "default-deny must declare BOTH Ingress and Egress policy types."
  }

  # 2. The allow policy carves the cloud metadata IP /32 out of an ip_block.except (a primary
  #    credential-theft vector blocked at the CNI layer).
  assert {
    condition = anytrue([
      for e in kubernetes_network_policy.allow.spec[0].egress :
      anytrue([
        for t in e.to :
        length(t.ip_block) > 0 ? contains(t.ip_block[0].except, "169.254.169.254/32") : false
      ])
    ])
    error_message = "allow policy must exclude the metadata IP 169.254.169.254/32 in an ip_block.except."
  }

  # 3. Egress opens ONLY DNS (53), the control-plane port, and the workload port — no other ports
  #    leak through the allowlist.
  assert {
    condition = alltrue(flatten([
      for e in kubernetes_network_policy.allow.spec[0].egress : [
        for p in e.ports : contains(["53", tostring(var.control_plane_port), tostring(var.workload_port)], tostring(p.port))
      ]
    ]))
    error_message = "allow egress must open ONLY DNS (53), the control-plane port, and the workload port."
  }

  # 4. Ingress to the workload is permitted on the workload port (so the namespace-wide deny does
  #    not strangle the workload's own serving traffic).
  assert {
    condition = anytrue(flatten([
      for i in kubernetes_network_policy.allow.spec[0].ingress : [
        for p in i.ports : tostring(p.port) == tostring(var.workload_port)
      ]
    ]))
    error_message = "allow policy must permit ingress to the workload port."
  }
}
