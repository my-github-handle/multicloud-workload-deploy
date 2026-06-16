# BYOC fast path

The cloud-agnostic entry point: **one `terraform apply`** against an existing Kubernetes cluster
(kubeconfig only, no cloud-provider credentials) produces a preflight-gated, secure, observable,
lifecycle-managed Workload. The shipped root is [`roots/agnostic-deploy`](../../../roots/agnostic-deploy);
use it directly or copy it into your own IaC repo to wire remote state.

It composes the five Layer-3 modules in order:

1. **preflight** — gate on the verdict (red blocks the plan) and derive the install tier.
2. **k8s-platform** — Tier A installs the operator + CRD (with the CRD-Established wait); Tier B is
   a no-op.
3. **k8s-security** — default-deny + metadata-block NetworkPolicies and the configurable
   PodSecurity floor.
4. **k8s-observability** — in-cluster ServiceMonitor + Grafana dashboard.
5. **workload** — Tier A Workload CR / Tier B `charts/workload`, from a single YAML spec.

The `kubernetes`/`helm`/`kubectl`/`external` providers are configured from the kubeconfig only —
there is **no cloud provider block**. The workload is supplied as one `workload_spec_yaml`
document; the namespace's allowed workload port is derived from it so they cannot drift.

- End-to-end walkthrough (on kind): [`verify-on-kind.md`](./verify-on-kind.md).
- Operator / workload day-2: [`workload-operator.md`](./workload-operator.md).
- Tier B (Helm-only): [`helm-only-tier-b.md`](./helm-only-tier-b.md).
- Module inputs/outputs: see the `variables.tf` / `outputs.tf` of the Layer-3 modules under
  `modules/` (`k8s-platform`, `k8s-security`, `k8s-observability`, `workload`, `preflight`).
