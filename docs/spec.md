# BYOC Multi-Cloud Workload Deployment — Product Spec

**Owner:** Infrastructure / Platform
**Status:** Draft for review
**Version:** 1.0

> Companion documents: [`architecture.md`](./architecture.md) (system shape) ·
> [`design.md`](./design.md) (detailed engineering design).

---

## 1. Problem & Objective

We ship a containerized workload into customer
environments as a **BYOC (Bring Your Own Cloud) product**. We need a production-ready,
**modular** infrastructure deliverable that:

- Runs the workload on Kubernetes via Terraform across **AWS, GCP, and Azure**.
- Treats **BYOC as the primary path**: most large customers already have a cluster, VPC,
  and encryption keys — we attach and deploy quickly. Greenfield (we provision everything)
  is the fallback.
- Separates a central, **company-operated control plane** from **satellite data planes**
  that run inside each customer's cloud, connected by an **outbound-only** link. Customer
  code and financial data never leave the customer cloud.
- Provides **reusable Terraform building blocks**: networking, IAM, KMS/encryption, secrets.
- Supports **provision-or-BYO** independently for network, cluster, and encryption key.
- Manages the workload lifecycle through a **cloud-agnostic Kubernetes operator** (deploy,
  rollout, autoscaling, health) with first-class **observability**.
- Enforces a security posture suitable for **untrusted/customer code touching financial
  data**: default-deny egress, firewall allowlisting, network policy, metadata-endpoint
  blocking, audit-grade observability.
- Gates every deployment with a **layered preflight** that validates prerequisites
  bottom-up (identity → keys → secrets → network → k8s → workload) and fails fast.

---

## 2. Success Criteria

- **Primary:** a solutions engineer can run **one `terraform apply`** against a customer's
  existing cluster and get a secure, observable, lifecycle-managed deployment — or, for
  greenfield, provision the full stack with the same downstream experience.
- A customer who already owns a VPC, cluster, or encryption key can bring any subset of them
  and have us provision the rest, with no change to the workload-layer experience.
- The same workload deploys to EKS, GKE, and AKS with only per-cloud values differing.
- No deployment proceeds silently when a security prerequisite (network policy support,
  metadata blocking, encryption at rest) is unmet — the preflight surfaces it.
- An immutable, customer-owned network audit trail (cloud VPC flow logs) is enabled on
  **every** deployment path, independent of the cluster's CNI.
- Every identity we create (deploy-time and runtime) carries the **minimum** permission set
  for its job — scoped to specific resources, no wildcards — and a customer can review the
  exact policy document before granting it.
- Customer code and financial data never traverse the control-plane boundary.

---

## 3. Scope & Non-Goals (YAGNI)

**In scope:** reusable per-cloud Terraform building blocks (network, IAM, KMS, secrets),
per-cloud cluster provisioning, a cloud-agnostic workload operator packaged as a Helm chart,
the satellite-side control-plane connect agent, layered preflight, and the security and
observability posture for untrusted-code/financial-data workloads. **Includes concrete,
least-privilege IAM policy documents per cloud** — both the deploy-time identity and the
runtime workload identity — derived from the exact action set each module requires, shipped
as versioned artifacts in the `iam` modules and asserted by preflight.

**Out of scope:**

- Not a generic multi-workload PaaS. One `Workload` CRD reconciling one workload shape.
- No compliance *certification* scope; we document how controls map to PCI-style
  requirements, we do not certify.
- `<cloud>-full` greenfield provisioning is delivered to production standards as the secondary
  path; the BYOC fast path, operator, and preflight are the primary surface.
- Traffic-shifting/canary logic is **delegated** (the operator can emit an Argo Rollouts
  `Rollout`) and **capability-gated**. Kubernetes-native `RollingUpdate` is the
  always-available default. (See [`design.md`](./design.md) — Rollout Strategy.)
- The control plane is designed for, but this deliverable does not build out, full
  fleet-management UI/billing. We define the satellite boundary, the connection contract, and
  the satellite-side agent; central services are interface-level only.

---

## 4. BYO-Cluster Shared-Responsibility Contract

When a customer brings their own cluster, we don't own the node pools, CNI, or control-plane
configuration. Responsibilities split as follows. (Mechanics of detection and enforcement
live in [`design.md`](./design.md) — Preflight.)

| Capability | Full-provision (we own) | BYO-cluster (customer owns) |
|---|---|---|
| K8s version / API compat | We set it | Preflight asserts min version |
| CNI w/ NetworkPolicy | We install | Required — detected; absent → documented gap |
| Workload Identity | We configure | Required enabled; we bind our SA |
| Private nodes / no public IP | We enforce | Customer responsibility; we assert & warn |
| Edge egress firewall | We provision (`network`) | Customer's VPC; we provide policy templates |
| Control-plane connectivity | We allowlist the FQDN | Customer allowlists one outbound FQDN |
| Argo Rollouts (cluster-scoped) | We install (optional) | Attach if present; else degrade to RollingUpdate |
| PodSecurity / runtime isolation | We set defaults | We apply namespaced PSA + policies |
| CRD + operator (cluster-scoped) | We install | Cluster-scoped bootstrap (Tier A); else operator-less namespaced manifests (Tier B) |
| Namespace, RBAC, observability, connect-agent | We install | We install (namespace-scoped) |

- **Always controlled even in BYO:** our namespace, connect-agent, in-cluster default-deny
  NetworkPolicy for our pods, namespaced PodSecurity, namespace-scoped RBAC, and observability
  scoped to our workload.
- **Degraded gracefully:** the operator + CRD (operator-less namespaced manifests under
  namespace-only permissions — see [`design.md`](./design.md) §2.5), node-level isolation,
  VPC-edge egress firewall, and cluster-scoped Argo Rollouts — documented gap + fallback,
  never a silent assumption of safety.

---

## 5. Deferred Items & Future Enhancements

**Deferred (out of scope for this deliverable):**

- Control-plane transport specifics (e.g. gRPC stream vs. reverse tunnel) and enrollment/cert
  issuance flow for the connect-agent. We define the **boundary and contract** — outbound-only
  mTLS, one allowlisted FQDN, pull-based desired state, local-first telemetry (see
  [`architecture.md`](./architecture.md) §4) — but the concrete wire protocol and enrollment
  flow are deferred to the control-plane workstream.

**Future enhancements (post-deliverable):**

- **Marketplace packaging (AWS / GCP / Azure).** Wrap the satellite deploy as a per-cloud
  marketplace listing with entitlement and metered-billing integration, so customers can
  subscribe and provision through their cloud's marketplace. The satellite is already the
  installable unit; this adds listing artifacts, entitlement checks, and usage reporting on
  top of the existing entry points.
