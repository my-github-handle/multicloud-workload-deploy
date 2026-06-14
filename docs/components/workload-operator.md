# Cloud-Agnostic Core — Operator & Workload Charts

**Status:** Implemented
**Layer:** 3 (cloud-agnostic Kubernetes layer)

> Parent documents: [`../architecture.md`](../architecture.md) (§2 Core Conventions, §6 Repository
> Structure, Layer 3) · [`../design.md`](../design.md) (§2 Layer 3, §2.1 Operator, §2.5 Install
> model) · [`../spec.md`](../spec.md) (§2 Success Criteria).
>
> This document is the component-level architecture and design for the operator and the two Helm
> charts. It refines the parent design for this component; where they differ the parent wins.
>
> Related component: [`preflight-checker.md`](./preflight-checker.md) — the staged gate that runs
> before this operator is installed (its Stage 4 selects the Tier A vs Tier B install).

---

## 1. What this component is

The cloud-agnostic core is the part of the system that runs **inside every customer cluster**,
identically on EKS/GKE/AKS, regardless of how the cluster, network, or keys were provisioned. It
has three pieces:

- **`Workload` CRD + operator** (`operator/`) — a namespaced custom resource and a
  controller-runtime reconciler that turns one `Workload` into a running, autoscaled, governed
  application.
- **`charts/workload`** — the single source of the workload's child objects (Deployment, Service,
  HPA, PDB, NetworkPolicy, optional Ingress).
- **`charts/workload-operator`** — the install chart for the CRD, controller, and
  namespace-scoped RBAC.

It deliberately excludes cloud building blocks, the preflight checker, Terraform wrappers, and the
connect-agent — those are separate components layered around this core.

---

## 2. The single-source render contract (anti-drift)

The defining design decision: the workload's child objects are defined **once**, in
`charts/workload`, and rendered by **two** independent paths that must never diverge
([`../design.md`](../design.md) §2.5):

- **Operator path** — the controller renders the embedded chart in-process
  (`operator/internal/render`, Helm SDK + `go:embed`) and owns the resulting objects, adding
  lifecycle (status, drift correction, governed labels).
- **Terraform path** — `helm template` / `helm_release` renders the same chart on disk for the
  operator-less install tier.

```
                charts/workload  (one chart, one values.schema.json)
                  /                              \
   operator render (in-process)          helm template / helm_release
        = Tier A (lifecycle)                   = Tier B (terraform-driven)
                  \                              /
              identical child objects (asserted by a parity test)
```

A render-parity test (`operator/internal/render/parity_test.go`) renders the same input through
both paths and asserts identical object sets, specs, **and** labels — and that the chart emits no
cluster-scoped object. Templates never reference `.Release.*`, so release name cannot perturb
output; a CI grep gate enforces this.

### Why a repo-root embed

`go:embed` resolves relative to the embedding file's directory and cannot escape it, so the chart
can only be embedded by a file at the repository root. The module root package (`chartassets.go`)
does the embed; `operator/internal/chartfs` re-exports it as the stable import surface. This is
why there is a single Go module rooted at the repository root.

---

## 3. The Workload API

One `kind: Workload` (`workload.ops.dev/v1`), namespaced. The spec is intentionally narrow — one
workload shape, not a generic PaaS:

| Field | Purpose |
|---|---|
| `image`, `port` | the container and the port it serves |
| `autoscale` | HPA bounds (`minReplicas`, `maxReplicas`, `targetCPUUtilization`); CEL-validated so max ≥ min |
| `rolloutStrategy` | `RollingUpdate` (default) or `Canary` (surfaced as a degraded condition until the rollout component lands) |
| `livenessProbe`, `readinessProbe` | optional HTTP probes |
| `resources` | container requests/limits |
| `securityContext`, `podSecurityContext` | optional overrides of the hardened default |
| `ingressClass`, `ingress` | optional Ingress routing host/path to the Service |

Status carries `conditions` (`Ready`, `RolloutDegraded`), `observedGeneration`, and
`readyReplicas`.

---

## 4. Reconcile behavior

The reconciler (`operator/internal/controller`) renders the chart for the `Workload`, then
applies each child object with:

- **Controller owner references** — every child is owned by the `Workload`, so deletion
  cascade-collects them and the `Owns()` watch re-reconciles on child changes.
- **Governance labels** — `app.kubernetes.io/{name,instance,part-of,managed-by}` on every child so
  a workload's resources are identifiable and operable as one unit; labels/annotations are
  reconciled on update, not just at create.
- **Immutable-field preservation** — Service `clusterIP` and Deployment `selector` survive updates
  (`CreateOrUpdate` with a mutate function), so applies don't churn or get rejected.
- **A single status patch** per reconcile (`Status().Patch(MergeFrom)`); conflicts propagate so the
  request requeues. `readyReplicas` converges from the live Deployment via the `Owns()` watch plus
  a `RequeueAfter` safety net.

`Canary` is never silently ignored: the chart renders `RollingUpdate` and the reconciler sets
`RolloutDegraded=True / CanaryUnsupported` so the unsupported request stays visible.

---

## 5. Security posture (in-cluster floor)

This component carries the portable, CNI-independent security floor that holds on any conformant
cluster ([`../design.md`](../design.md) §2.2):

- **Hardened pod defaults** — non-root, no privilege escalation, read-only root filesystem, all
  capabilities dropped, `RuntimeDefault` seccomp. Overridable per `Workload` for images that
  cannot satisfy them, so the default stays strict while remaining usable.
- **Default-deny NetworkPolicy** plus an allow policy that permits the workload's own port and DNS,
  and **blocks egress to the cloud metadata IP** `169.254.169.254` (a primary credential-theft
  vector).
- **Namespace-scoped operator** — the controller's cache is restricted to the watched namespace and
  its RBAC is a `Role` (never a `ClusterRole`). The only cluster-scoped object is the CRD itself.

---

## 6. Install model (capability tiers)

- **Tier A (operator).** The CRD + RBAC are installed once; the controller then runs
  namespace-scoped and reconciles `Workload` resources with full lifecycle. The operator may run in
  a different namespace from the workloads it manages — its Role/RoleBinding are created in the
  **watched** namespace, the ServiceAccount in the install namespace.
- **Tier B (operator-less).** Where no cluster-scoped object may be created, Terraform renders the
  same `charts/workload` directly into the granted namespace. The workload still runs within the
  security posture; the `Workload` abstraction and operator-driven lifecycle are not available.

Both tiers render the identical chart, so they cannot drift — only lifecycle ownership differs.

---

## 7. Packaging

The component ships as a **product**, not just source:

- **`charts/workload-operator`** — standalone `helm install`-able: CRD (under `crds/`), controller
  Deployment, namespace-scoped Role/RoleBinding, metrics Service, optional ServiceMonitor,
  ServiceAccount-level `imagePullSecrets` for private registries.
- **`charts/workload`** — standalone-renderable child-object chart with one `values.schema.json`
  shared as the input contract between the CR spec and Terraform variables.
- **Operator image** — distroless, non-root, cross-compiled; built and published from the repo
  root so the embedded chart is included.
- Chart READMEs are generated from the chart metadata (helm-docs); chart versions are pinned per
  release.

---

## 8. Testing

See [`../../test/README.md`](../../test/README.md) for the local-vs-real-world split.

- **Local** — unit tests (render, parity) and envtest reconcile specs (child creation, ownership,
  labels, status convergence, immutable-field preservation, Canary degradation, CEL validation,
  Ingress, HPA wiring). Coverage gate ≥ 80% on the operator logic.
- **Real-world** — `test/e2e` drives the installed operator on a live cluster end-to-end.
