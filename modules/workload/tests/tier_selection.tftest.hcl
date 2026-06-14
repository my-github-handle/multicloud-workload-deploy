# Plan-time assertions that the workload module selects the right resource per install_tier, taking
# an identical spec_yaml input. Run with `terraform test` from modules/workload. command = plan so
# no cluster is required. Tier A uses kubectl_manifest (raw YAML, no plan-time CRD schema
# discovery), so the Tier A plan succeeds offline. Field assertions read the CR back via
# yamldecode(yaml_body) and the helm values via yamldecode(values[0]).
#
# The single spec_yaml source feeds both tiers, so the CR spec and the chart values cannot drift —
# this test proves selection + that the Tier B values cover the spec fields. The full Tier A vs
# Tier B render-parity test is owned by the core render package, not duplicated here.

variables {
  name      = "demo"
  namespace = "demo-ns"
  spec_yaml = <<-YAML
    image: nginx:1.27
    port: 8080
    autoscale:
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilization: 70
  YAML
}

run "tier_a_plans_workload_cr_only" {
  command = plan

  variables {
    install_tier = "A"
  }

  assert {
    condition     = length(kubectl_manifest.workload_cr) == 1
    error_message = "Tier A must plan exactly one Workload CR (kubectl_manifest.workload_cr)."
  }
  assert {
    condition     = length(helm_release.workload) == 0
    error_message = "Tier A must NOT plan a charts/workload helm_release."
  }
  assert {
    condition     = yamldecode(kubectl_manifest.workload_cr[0].yaml_body).apiVersion == "workload.ops.dev/v1"
    error_message = "Tier A Workload CR must use apiVersion workload.ops.dev/v1."
  }
  assert {
    condition     = yamldecode(kubectl_manifest.workload_cr[0].yaml_body).kind == "Workload"
    error_message = "Tier A resource must be kind Workload."
  }
  assert {
    condition     = yamldecode(kubectl_manifest.workload_cr[0].yaml_body).spec.image == "nginx:1.27"
    error_message = "Workload CR spec.image must mirror the shared spec_yaml input."
  }
  assert {
    condition     = yamldecode(kubectl_manifest.workload_cr[0].yaml_body).spec.autoscale.maxReplicas == 10
    error_message = "Workload CR spec.autoscale.maxReplicas must mirror the shared spec_yaml input."
  }
  assert {
    condition     = output.tier == "A"
    error_message = "tier output must report A."
  }
}

run "tier_b_plans_helm_release_only" {
  command = plan

  variables {
    install_tier = "B"
  }

  assert {
    condition     = length(helm_release.workload) == 1
    error_message = "Tier B must plan exactly one charts/workload helm_release."
  }
  assert {
    condition     = length(kubectl_manifest.workload_cr) == 0
    error_message = "Tier B must NOT plan a Workload CR."
  }
  assert {
    condition     = helm_release.workload[0].name == "demo"
    error_message = "Tier B helm_release name must mirror the shared name input."
  }
  assert {
    condition     = output.tier == "B"
    error_message = "tier output must report B."
  }

  # The Tier B helm values must carry the identity + every spec field from spec_yaml, plus the
  # chart-only PDB knob — proving no spec field is dropped on the Tier B path and that the same
  # source feeds both tiers.
  assert {
    condition = alltrue([
      for k in ["name", "namespace", "image", "port", "autoscale", "pdb"] :
      contains(keys(yamldecode(helm_release.workload[0].values[0])), k)
    ])
    error_message = "Tier B helm values must include name, namespace, image, port, autoscale, and pdb."
  }
  assert {
    condition     = yamldecode(helm_release.workload[0].values[0]).autoscale.maxReplicas == 10
    error_message = "Tier B helm values.autoscale must mirror the shared spec_yaml input (no drift)."
  }
}

# Optional spec fields (probes, security contexts) supplied via spec_yaml must flow through to both
# tiers untouched.
run "optional_spec_fields_flow_through" {
  command = plan

  variables {
    install_tier = "B"
    spec_yaml    = <<-YAML
      image: nginx:1.27
      port: 80
      autoscale:
        minReplicas: 2
        maxReplicas: 5
        targetCPUUtilization: 70
      livenessProbe:
        path: /healthz
        port: 80
      podSecurityContext:
        runAsNonRoot: false
      securityContext:
        readOnlyRootFilesystem: false
    YAML
  }

  assert {
    condition     = yamldecode(helm_release.workload[0].values[0]).livenessProbe.path == "/healthz"
    error_message = "Tier B values must carry the livenessProbe from spec_yaml."
  }
  assert {
    condition     = yamldecode(helm_release.workload[0].values[0]).podSecurityContext.runAsNonRoot == false
    error_message = "Tier B values must carry podSecurityContext overrides from spec_yaml."
  }
}
