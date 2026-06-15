# Operations Guide

How to operate the BYOC multi-cloud workload deployment product. This is the whole-project entry
point; component-specific runbooks are linked below.

> Architecture: [`../architecture.md`](../architecture.md) · Design: [`../design.md`](../design.md)
> · Components: [`../components/`](../components/)

---

## Pick your path

```
Do you already have a Kubernetes cluster?
├─ YES → BYOC fast path: a single `terraform apply` of the Layer-3 deploy onto your cluster.
│         Walkthrough: common/verify-on-kind.md (generalizes to any EKS/GKE/AKS).
└─ NO  → Greenfield: provision the cluster + network/identity/encryption, then deploy.
          AWS: aws/deploy.md   ·   GCP: gcp/deploy.md   ·   Azure: azure/deploy.md.

Can the deploy identity create a cluster-scoped CRD + ClusterRole?
├─ YES → Tier A (operator): common/workload-operator.md
└─ NO  → Tier B (Helm-only, namespace-scoped): common/helm-only-tier-b.md

Every path runs the preflight gate first: common/preflight.md.
```

## Map of operations docs

**Common** (cloud-agnostic — apply to every path):

| Doc | Scope |
|---|---|
| This file | Entry point: pick-your-path, prerequisites, day-2, governance |
| [`common/preflight.md`](./common/preflight.md) | The preflight gate — running it, reading the report, per-concern applicability |
| [`common/workload-operator.md`](./common/workload-operator.md) | Tier A: operating the operator and `Workload` resources |
| [`common/helm-only-tier-b.md`](./common/helm-only-tier-b.md) | Tier B: deploying `charts/workload` with Helm only (no operator/CRD) |
| [`common/byoc-deploy.md`](./common/byoc-deploy.md) | BYOC fast path (`_agnostic-deploy`): one apply onto an existing cluster |
| [`common/verify-on-kind.md`](./common/verify-on-kind.md) | BYOC fast-path walkthrough on kind (no cloud account) |
| [`examples/`](./common/examples) | Runnable, copy-pasteable `Workload` manifests |

**Per cloud** (greenfield: provision infra + deploy, end to end):

| Doc | Scope |
|---|---|
| [`aws/deploy.md`](./aws/deploy.md) | AWS greenfield (`aws-full`): provision → single apply → operating notes → BYO variations → teardown; `aws/examples/` |
| [`gcp/deploy.md`](./gcp/deploy.md) | GCP greenfield (`gcp-full`): provision → single apply → operating notes → BYO variations → teardown; `gcp/examples/` |
| [`azure/deploy.md`](./azure/deploy.md) | Azure greenfield (`azure-full`): provision → single apply → operating notes → BYO variations → teardown; `azure/examples/` |

---

## Prerequisites

- `kubectl`, `helm` (v3), and access to a target cluster (`KUBECONFIG`).
- For building/publishing the operator: `go` (1.26), `docker` (with buildx), `mage`.
- The operator image reachable by the cluster — public, or private with an image pull secret.

---

## Deploy paths

The product has two entry shapes (see [`../architecture.md`](../architecture.md) §5):

- **Greenfield** — provision the cloud infra (network, identity, encryption, cluster) *and* deploy,
  in one Terraform root (single apply). Per-cloud, end to end:
  **[`aws/deploy.md`](./aws/deploy.md)** · **[`gcp/deploy.md`](./gcp/deploy.md)** ·
  **[`azure/deploy.md`](./azure/deploy.md)**. The preflight gate runs automatically and blocks
  `apply` on a red verdict.

- **BYOC** — you already have a cluster; a single `terraform apply` (or staged Helm install) lays
  down the cloud-agnostic Layer-3 deploy. The shared sequence:

  1. **Preflight** — `mage preflightBuild` then run the gate; see [`common/preflight.md`](./common/preflight.md).
  2. **Install** — **Tier A** (operator + CRD): [`common/workload-operator.md`](./common/workload-operator.md);
     or **Tier B** (Helm-only, namespace-scoped): [`common/helm-only-tier-b.md`](./common/helm-only-tier-b.md).
  3. **Deploy a `Workload`** — `kubectl apply -f common/examples/workload-basic.yaml` (see [`examples/`](./common/examples)
     for probes/ingress/root-image/autoscale variants).
  4. **Verify** — `Ready=True` + the full child set; every child carries
     `app.kubernetes.io/{name,instance,part-of,managed-by}` for fleet-wide querying.

  A complete copy-pasteable BYOC walkthrough is [`common/verify-on-kind.md`](./common/verify-on-kind.md).

---

## Day-2 operations

| Task | How |
|---|---|
| Update image / scale bounds | edit the `Workload` (`kubectl edit workload <name>`); the operator reconciles |
| Inspect status | `kubectl get workload <name> -o yaml` → `.status.conditions`, `.status.readyReplicas` |
| Diagnose a stuck workload | see [`common/workload-operator.md`](./common/workload-operator.md) → Troubleshooting |
| Upgrade the operator | `helm upgrade op charts/workload-operator …` (re-applies the CRD) |
| Uninstall a workload | `kubectl delete workload <name>` — children are garbage-collected via owner refs |
| Uninstall the operator | `helm uninstall op -n workload-system` (CRD removal is manual and deletes all Workloads) |

---

## Security & governance notes

- Workloads run under a hardened pod security context by default; relaxing it is explicit and
  per-workload (`spec.securityContext` / `spec.podSecurityContext`).
- Each workload's pods are default-deny with the cloud metadata endpoint blocked.
- Resources are governed as a unit via the `app.kubernetes.io/instance=<name>` label selector.
