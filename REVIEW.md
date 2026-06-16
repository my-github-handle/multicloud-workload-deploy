# Reviewer's guide — E2B SRE assignment

This maps the assignment requirements to where each is implemented. The deliverable is a
**BYOC multi-cloud product**, not a single deployment: the same containerized workload
(`ghcr.io/e2b-dev/sre-interview:latest`, HTTP on `:8080`, internal on `:9090`) deploys to **AWS,
GCP, and Azure** through reusable Terraform modules and a cloud-agnostic Kubernetes operator.

Start here: [`README.md`](README.md) · [`docs/spec.md`](docs/spec.md) ·
[`docs/architecture.md`](docs/architecture.md) · [`docs/design.md`](docs/design.md).

## Requirement → where it lives

| Assignment ask | Where | Notes |
|---|---|---|
| **Terraform, ≥ 2 cloud providers** | `modules/aws/`, `modules/gcp/`, `modules/azure/` | All **three** clouds — full parity (network, kms, iam, secrets, cluster + resolvers, preflight). |
| **Kubernetes deployment** | `operator/`, `charts/workload`, `charts/workload-operator` | A workload operator (CRD-driven) renders the Deployment/Service/HPA/PDB; Helm-only Tier B is the fallback. |
| **Reusable Terraform modules** | `modules/<cloud>/*` + cloud-agnostic `modules/k8s-*`, `modules/workload` | Each module is provision-or-BYO with a stable output interface; the create-vs-lookup branch is isolated in `*-resolver` modules. |
| **Terraform state** | `roots/agnostic-deploy` + per-cloud `roots/<cloud>-full/{phase1-infra,phase2-deploy}` | Shipped runnable roots use local state by default; production users wire cloud backends per runbook. |
| **Health checks** | `workload_spec_yaml.livenessProbe` / `readinessProbe` | Wired onto the Deployment by the operator/chart. |
| **Rollout strategy** | `Workload` CR `rolloutStrategy` (RollingUpdate; Argo Rollouts canary detected by preflight) | Default RollingUpdate; chart sets surge/unavailable. |
| **Autoscaling** | `workload_spec_yaml.autoscale` → HPA (min/max/targetCPU) + PDB | Example: 2–5 replicas, 70% CPU. |
| **Customer has a VPC / wants us to create one** | `network_mode = provision \| byo` (+ `byo_*`) | Independent toggle per concern; preflight self-gates BYO as amber, not red. |
| **Customer has a cluster / wants us to provision** | `cluster_mode = provision \| byo`; BYOC fast path | BYOC (existing cluster) is the **primary** path: a single Layer-3 `terraform apply`. |
| **Production-grade: reliability, security, portability** | see "Security posture" in [`README.md`](README.md) | default-deny egress firewall, private clusters, CMK encryption, workload identity (no static keys), wildcard-free roles, immutable audit floor, Cilium + NetworkPolicy floor. |
| **Packaging (product offering)** | `release/` BOM + `mage release:*` (cosign/SBOM provenance) | Operator image (by digest) + both charts + module set pinned in a versioned BOM. |
| **Preflight / safety** | `operator/cmd/preflight` + `operator/internal/cloud/<cloud>` + `modules/preflight` | Staged gate (identity/kms/secrets/egress/k8s) blocks `apply` on a red verdict. |

## Deploy the assignment image

The greenfield phase-2 examples deploy `ghcr.io/e2b-dev/sre-interview:latest` end to end:
[`roots/azure-full/phase2-deploy/terraform.tfvars.example`](roots/azure-full/phase2-deploy/terraform.tfvars.example)
(AWS/GCP have equivalent phase-2 examples). The older operations examples carry the same workload
shape:
[`docs/operations/azure/examples/greenfield.tfvars.example`](docs/operations/azure/examples/greenfield.tfvars.example)
(AWS/GCP have equivalent `examples/`). Because the image runs as **root** with a writable
filesystem, the examples set the namespace PSA to `baseline` and a root-compatible
`securityContext` (still drops ALL caps + RuntimeDefault seccomp) — the hardened chart defaults
assume a non-root image, and this is the documented per-workload override.

Per-cloud, end to end:
[`docs/operations/aws/deploy.md`](docs/operations/aws/deploy.md) ·
[`docs/operations/gcp/deploy.md`](docs/operations/gcp/deploy.md) ·
[`docs/operations/azure/deploy.md`](docs/operations/azure/deploy.md).

## What was validated

Each cloud's modules pass offline gates (`terraform validate`, resolver-parity + least-privilege
golden `terraform test`), the operator has unit/envtest coverage (≥ 80%), and the staged preflight
binary has table-driven per-cloud provider tests. Azure was additionally **live-validated** on a
real AKS cluster: infra apply followed by Layer-3 deploy → Workload `Ready=True`, pods on the CNI
overlay, in-cluster HTTP 200 through the NetworkPolicy floor, Cilium dataplane.
