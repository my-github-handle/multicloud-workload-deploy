# Multi-Cloud Workload Deploy

A **BYOC (Bring Your Own Cloud)** product for shipping a containerized workload onto Kubernetes
across **AWS, GCP, and Azure** — with a hardened network, customer-managed encryption,
least-privilege workload identity, a preflight gate, and a cloud-agnostic operator that runs the
workload identically everywhere.

Two entry shapes:

- **BYOC fast path** (primary) — you already have a cluster; a single `terraform apply` lays down
  the cloud-agnostic Layer-3 deploy (operator, security, observability, workload).
- **Greenfield** (`<cloud>-full`) — provision cloud infra in phase 1, then run preflight and deploy
  Layer 3 in phase 2 against the live cluster.

> Specification & design: [`docs/spec.md`](docs/spec.md) ·
> [`docs/architecture.md`](docs/architecture.md) · [`docs/design.md`](docs/design.md).

---

## Layers

| Layer | What | Where |
|---|---|---|
| **1 — Building blocks** | `network`, `kms`, `iam`, `secrets` (+ GCP `project`); provision-or-BYO | `modules/<cloud>/` |
| **2 — Per-cloud platform** | hardened private `cluster` (EKS/GKE/AKS) + `cluster-resolver` (uniform auth) | `modules/<cloud>/` |
| **3 — Cloud-agnostic Kubernetes** | the operator + `k8s-platform` / `k8s-security` / `k8s-observability` / `workload` | `modules/`, `operator/`, `charts/` |
| **4 — Composition** | `agnostic-deploy` (BYOC) and `<cloud>-full` (greenfield) roots | `roots/` |
| **Cross-cutting** | the staged **preflight** gate (Go binary + per-cloud `cloud.Provider`) | `operator/cmd/preflight`, `operator/internal/cloud/<cloud>` |
| **5 — Packaging** | BOM-versioned bundle (operator image + charts + modules) with provenance | `release/`, `magefile_release.go` |

Each Layer-1/2 module is a focused, provision-or-BYO building block exposing a stable output
interface; the single create-vs-lookup branch is isolated in the matching `*-resolver` module. The
resolvers emit uniform interfaces (`{vpc_id, subnet_ids, egress_path_ref}` for network,
`{endpoint, ca, auth}` for cluster) so Layer 3 is cloud-agnostic.

---

## Repository layout

```
charts/            workload + workload-operator Helm charts
modules/
  aws/  gcp/  azure/   per-cloud Layer-1/2 building blocks
  k8s-platform/ k8s-security/ k8s-observability/ workload/ preflight/   cloud-agnostic Layer 3
operator/          the workload operator + CRD; cmd/preflight binary; internal/cloud/<cloud> providers
docs/              spec, architecture, design, per-cloud architecture + operations runbooks
roots/             runnable Terraform entry points: BYOC and two-phase greenfield roots
release/           BOM template + release tooling
test/              e2e tests + runbooks
```

Greenfield composition roots (`<cloud>-full`) and the BYOC `agnostic-deploy` root are shipped as
runnable reference entry points under [`roots/`](roots/). Customers can run them directly for a
standard install or copy them into their own IaC repo to wire remote backends/state policy.

---

## Per-cloud guides

| Cloud | Architecture | Greenfield runbook |
|---|---|---|
| AWS | [`docs/architecture/aws.md`](docs/architecture/aws.md) | [`docs/operations/aws/deploy.md`](docs/operations/aws/deploy.md) |
| GCP | [`docs/architecture/gcp.md`](docs/architecture/gcp.md) | [`docs/operations/gcp/deploy.md`](docs/operations/gcp/deploy.md) |
| Azure | [`docs/architecture/azure.md`](docs/architecture/azure.md) | [`docs/operations/azure/deploy.md`](docs/operations/azure/deploy.md) |

Cloud-agnostic operations (preflight gate, operator/workload lifecycle, BYOC walkthrough) live
under [`docs/operations/`](docs/operations/README.md).

---

## Security posture

- **Default-deny egress** at the cloud edge (AWS Network Firewall / GCP Cloud NGFW / Azure
  Firewall) with an FQDN/CIDR allowlist — holds regardless of the in-cluster CNI.
- **Hardened private clusters** — no public node IPs; private API endpoint by default.
- **Customer-managed encryption** — a rotating CMK (KMS / Cloud KMS / Key Vault) for secrets and
  cluster/disk encryption.
- **Workload identity, no static keys** — IRSA (AWS) / Workload Identity (GCP) / federated UAMI
  (Azure), bound to a wildcard-free, resource-scoped custom role (no Owner/Contributor/admin).
- **Cilium dataplane on greenfield** (Cilium ENI on EKS, Dataplane V2 on GKE, Azure CNI Overlay +
  Cilium on AKS) plus a portable Kubernetes `NetworkPolicy` floor (default-deny + metadata-IP
  block) everywhere.
- **Always-on, immutable audit floor** — cloud flow logs to a customer-owned, retention-locked
  sink (S3 Object-Lock / Cloud Logging / WORM Storage), independent of the cluster.

---

## Build & test

The build is driven by [mage](https://magefile.org/) (`magefile.go`):

```bash
mage preflightBuild   # build operator/bin/preflight
mage test             # unit + envtest with the coverage gate (>= 80%)
mage lintCharts       # helm chart lint
mage verify           # test + lintCharts (default target)
mage dockerBuild      # build/push the operator image
```

Terraform modules are gated per-module (`terraform fmt -check`, `init -backend=false`, `validate`)
and `terraform test` for resolver parity, least-privilege golden tests, and CSI shape assertions.
Real-world e2e tests live under [`test/`](test/README.md) (build-tagged, run against a live
cluster/cloud).

**Toolchain:** Go 1.26 · Terraform ≥ 1.7 · Helm 3 · kubectl · the cloud CLI (`aws` / `gcloud` /
`az`, plus `kubelogin` for Entra-only AKS).

---

## Status

| Cloud | Building blocks | Greenfield root | Real preflight provider |
|---|---|---|---|
| AWS | ✅ | ✅ `roots/aws-full/{phase1-infra,phase2-deploy}` | ✅ |
| GCP | ✅ | ✅ `roots/gcp-full/{phase1-infra,phase2-deploy}` | ✅ |
| Azure | ✅ | ✅ `roots/azure-full/{phase1-infra,phase2-deploy}` | ✅ |
