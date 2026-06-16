# Preflight Checker

**Status:** Implemented (cloud stages 0–3 via a provider interface with AWS/GCP/Azure providers;
Kubernetes stages 4–5 implemented for real.)
**Layer:** orchestration (invoked by the Terraform deploy paths before any resource is created)

> Parent documents: [`../design.md`](../design.md) (§3 Layered Preflight) ·
> [`../architecture.md`](../architecture.md) (§3 Tooling — Terraform-driven, no wrapper script) ·
> [`../spec.md`](../spec.md) (§2 — "no deployment proceeds silently when a prerequisite is unmet").
>
> Related component: [`workload-operator.md`](./workload-operator.md). Operations:
> [`../operations/common/preflight.md`](../operations/common/preflight.md).

---

## 1. What this component is

A tested Go binary (`operator/cmd/preflight`) that validates deploy prerequisites **bottom-up in
dependency order** and emits a single green/amber/red report. Terraform invokes it through an
`external` data source and gates `apply` on the verdict, so a customer never debugs a Kubernetes
failure that is really a missing IAM permission.

The six stages mirror the layer graph:

```text
Stage 0  identity & access   ─┐ cloud-facing (provider interface; real per-cloud later)
Stage 1  kms / keys           │
Stage 2  secrets backend      │
Stage 3  network / egress    ─┘
Stage 4  kubernetes infra    ─┐ real, via client-go against the target cluster
Stage 5  workload readiness  ─┘
```

---

## 2. Report model and verdict

- A **`CheckResult`** has a stable `id`, a `status` (green/amber/red), a `message`, and optional
  `remediation`. The stable IDs (e.g. `iam.missing`, `egress.controlplane_fqdn`, `k8s.installtier`)
  are a contract the per-cloud providers and the Terraform module key on.
- A **`Stage`** folds its results (red if any red, else amber if any amber, else green) and carries
  a `Blocking` flag.
- **Verdict** = red if any *blocking* stage is red, else amber if any blocking stage is amber, else
  green. Skipped stages don't count.

### Blocking-mask (full / greenfield mode)

In `<cloud>-full/phase2-deploy`, cloud stages (0–3) cover resources created by phase 1, so they're
marked **non-blocking**. A non-blocking stage keeps its *true* severity in the report (a disabled
BYO key still shows red) but contributes at most amber to the verdict and never short-circuits —
so the Kubernetes deploy target (stages 4–5) always runs. The report is never rewritten to hide a
real gap; only its gating effect is masked.

In agnostic / BYOC mode every stage blocks, and the first red short-circuits the rest (later stages
are marked `skipped`).

---

## 3. How the stages run

- **Cloud stages 0–3** call a `PreflightProvider` interface (identity / kms / secrets / egress).
  AWS/GCP/Azure providers and a configurable fake emit the same stable result IDs. Stage 3's
  `egress.controlplane_fqdn` is load-bearing — blocked means the connect-agent can't dial home, so
  it blocks the deploy.
- **Kubernetes stages 4–5** run for real via `client-go` against the target cluster: NetworkPolicy
  API, Cilium CRD detection, metrics-server, min K8s version, PodSecurity admission, **install-tier
  selection** (Tier A vs B vs red, via `SelfSubjectAccessReview`), Argo Rollouts
  presence+version+traffic-primitive, the Workload-Identity ServiceAccount binding, and namespace
  creatability.

---

## 4. The Terraform contract

The binary prints a **flat JSON object of string→string** (the only shape the hashicorp `external`
provider accepts):

```json
{ "verdict": "green|amber|red", "report_json": "<the full Report, JSON-encoded as a string>" }
```

It **always exits 0** when invoked by Terraform, even on internal failure (bad kubeconfig,
unimplemented `--cloud`): the error is surfaced as a red verdict carrying the message, so the gate
blocks on the verdict rather than aborting the plan with an opaque provider error. With no
kubeconfig a real run emits a **blocking red Stage 4** (the deploy target is unreachable — never a
false green). `--exit-on-red` (default off) is for standalone CLI use.

---

## 5. Why a binary, not a wrapper script

Terraform is the single driver; there is no `deploy.sh`. The checks reuse the operator's cloud
clients so preflight and the operator never diverge, and the binary is unit-tested and shipped in
the release tooling. The Terraform `preflight` module (the `external` data source + gating) is a
separate Layer-3 component.

---

## 6. Scope

**Implemented:** the staged runner, report/verdict, the cloud-stage interface + fake, the real
Kubernetes stages, and the binary + JSON contract.

**Deferred:** real per-cloud `PreflightProvider` implementations (with the cloud building blocks),
the Terraform `preflight` module (with the Layer-3 deploy path), credentialed image-pull probing,
and Stage-0 *excess*-permission detection (the blocking *missing*-permission check is implemented).

---

## 7. Testing

See [`../../test/README.md`](../../test/README.md). Local: table-driven verdict/runner tests, cloud
fixtures (stable egress IDs, control-plane-FQDN reachable/blocked), and **envtest**-backed K8s
stages (Cilium/Argo present-absent, version compatibility, workload-identity, install-tier). The
binary's flag/provider/emit paths and the external-data-source JSON contract are unit-tested.
