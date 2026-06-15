# BYOC Multi-Cloud Workload Deployment — Architecture

**Owner:** Infrastructure / Platform
**Status:** Draft for review
**Version:** 1.0

> Companion documents: [`spec.md`](./spec.md) (requirements & scope) ·
> [`design.md`](./design.md) (detailed engineering design) ·
> [`architecture/aws.md`](./architecture/aws.md) (AWS building blocks & `aws-full` greenfield) ·
> [`architecture/gcp.md`](./architecture/gcp.md) (GCP building blocks & `gcp-full` greenfield) ·
> [`architecture/azure.md`](./architecture/azure.md) (Azure building blocks & `azure-full` greenfield).

---

## 1. Layered Overview

A **layered, composable** footprint. Each layer is independently usable and independently
stated. BYO is a **composition choice** (don't call the provisioning module; pass in an ID),
never a tangle of `create_*` booleans inside modules.

```
┌─ Layer 5: Packaging & distribution ───────────────────────────┐
│  Marketplace packaging (future enhancement)                    │
│    └─ built on: BOM-versioned release (operator image +        │
│       charts + TF modules), cosign-signed, SBOM (§7)           │
├─ Layer 4: Composition (root configs / "live" envs) ───────────┤
│  Entry points:  _agnostic-deploy (BYOC fast path)             │
│                 <cloud>-full (greenfield fallback)            │
│  Toggles: BYO-VPC · BYO-cluster · BYO-key (independent)       │
├─ Layer 3: Cloud-agnostic Kubernetes layer ────────────────────┤
│  operator (CRD+controller) · connect-agent · security policies │
│  · observability · Workload CR (the workload image)           │
├─ Layer 2: Per-cloud platform modules ─────────────────────────┤
│  cluster (EKS/GKE/AKS, private/hardened) · cluster-resolver    │
├─ Layer 1: Reusable per-cloud building blocks ─────────────────┤
│  network (+egress/firewall) · iam (workload identity)          │
│  · kms (CMK/BYO key) · secrets                                 │
└────────────────────────────────────────────────────────────────┘
```

---

## 2. Core Conventions

- **Building blocks are the named reusables.** `network`, `iam`, `kms`, `secrets` are
  first-class Layer-1 modules, each with one job and a clean output interface, independently
  consumable by a customer who only needs one piece.
- **Least privilege is action-derived.** Each module declares the exact cloud API actions it
  performs; the `iam` module composes those into resource-scoped, wildcard-free policy
  documents (deploy-time + runtime identities) that ship as reviewable artifacts and are
  asserted by preflight. Policies can't drift from what the code does. Details in
  [`design.md`](./design.md) §1.2.
- **Resolver pattern.** Each cloud has thin resolvers (`network`, `cluster`, `kms` in BYO
  mode) that output a uniform interface — `{vpc_id, subnet_ids, egress_path_ref}`,
  `{endpoint, ca, auth}`, `{key_id/arn/resource_id}` — **whether the resource was created or
  looked up**. The single create-vs-lookup branch lives only in the resolver. Everything
  downstream receives identical inputs.
- **BYO = composition, not booleans.** "Bring your own VPC" means the root config does not
  call `network`; it feeds an existing ID into the resolver. BYO-VPC, BYO-cluster, BYO-key
  are independent toggles; a customer can BYO some and have us provision the rest.
- **Layer 3 is invariant.** The operator, security policies, observability, and Workload CR
  never know whether the cluster/VPC/key was provisioned by us or pre-existing. This is what
  makes the BYOC deploy experience consistent across all starting points and all three clouds.
- **State is layered.** Separate remote state per layer (network / cluster / workload) so a
  workload change never plans against the VPC. Backends are cloud-native: S3 + DynamoDB lock
  (AWS), GCS (GCP), azurerm (Azure).
- **Portable floor + detected enhancement.** Where a capability is cluster-scoped and may be
  customer-owned or restricted in BYO, we require a portable floor and treat the richer option
  as a preflight-detected enhancement with a perimeter backstop. This applies to the **CNI**
  (Cilium/identity-aware policy where it runs natively — GKE Dataplane V2 is Cilium, and EKS
  can add Cilium chaining on top of the VPC CNI; the portable Kubernetes `NetworkPolicy` floor
  everywhere else; cloud egress firewall as the FQDN backstop), to **network observability**
  (cloud VPC flow logs as the always-on, customer-owned audit floor; Hubble as the
  Cilium-gated detection layer), and to the **install model** (the operator + cluster-scoped
  CRD where permitted; plain namespaced manifests as the floor under namespace-only
  permissions). Details in [`design.md`](./design.md) §3.2–§3.5.

---

## 3. Tooling & Drivers

- **Terraform is the single driver.** Every deployment path — provisioning and workload
  install — is one `terraform apply`. No separate `kubectl`/`helm`/script step in the happy
  path; orchestration tooling stays out of the customer's hands. Preflight runs as a tested
  checker binary that Terraform invokes (via an `external` data source) and gates `apply` on —
  not a wrapper script (see [`design.md`](./design.md) §3).
- **The operator is packaged as a Helm chart** (`charts/workload-operator`). Terraform
  installs it via the **`helm` provider** (`helm_release`); the chart carries the operator's
  CRDs, controller Deployment, RBAC, ServiceMonitor, and the optional `connect-agent`. This
  gives a single versioned, standalone-installable artifact (a customer or GitOps pipeline can
  `helm install` it directly), while Terraform remains the orchestrator that wires it to
  resolved infra.
- **Layer 3 Terraform modules wrap Kubernetes resources** via the `kubernetes`/`helm`
  providers configured from the cluster-resolver outputs — so the same agnostic deploy works
  identically on EKS/GKE/AKS, BYO or greenfield.

---

## 4. Control & Data Plane Separation (Satellite Architecture)

The product is a **fleet of satellites**: the **data plane** runs inside each customer's
cloud (where untrusted code and financial data live), and a central **company-operated
control plane** manages the fleet. The two are joined by an **outbound-only** link so that
nothing ever connects *into* the customer environment.

```
        COMPANY-OPERATED CONTROL PLANE (central, our cloud)
        ┌──────────────────────────────────────────────┐
        │ fleet registry · satellite inventory/health   │
        │ desired-state API · release/version catalog   │
        │ aggregated observability                       │
        └───────────────▲────────────────────────────────┘
                        │  outbound-only mTLS — satellite dials home
                        │  (one allowlisted FQDN, NO inbound holes)
        ┌───────────────┴────────────────────────────────┐
        │ CUSTOMER CLOUD — SATELLITE (data plane)          │
        │  Workload operator + Workload CR (workload image)│
        │  + connect-agent (outbound tunnel)               │
        │  default-deny egress · private cluster · CMK     │
        │  untrusted code + financial data NEVER leave here│
        └──────────────────────────────────────────────────┘
```

### Principles

1. **Data plane is self-sufficient.** The satellite (operator + Workload + observability)
   reconciles **locally** and keeps the workload running even if the control-plane link is
   down. The control plane handles fleet management and is off the critical serving path, so
   the satellite operates independently inside the customer's cloud.

2. **Outbound-only connection (`connect-agent`).** A lightweight agent in the satellite
   namespace initiates a persistent **mTLS** connection *out* to the control plane.
   Consequences that reinforce the security posture:
   - Adds exactly **one allowlisted FQDN** to the egress firewall — no inbound rules, no
     public cluster endpoint, works behind customer NAT.
   - The control plane **never** initiates a connection inward — preserving
     private-cluster / no-public-IP.
   - Agent identity = the satellite's workload identity + a per-satellite enrollment
     certificate.

3. **Financial data never crosses the boundary.** Only **control-plane metadata** traverses
   the tunnel: satellite health, version/drift status, preflight reports, and
   **aggregated/redacted** telemetry. Raw logs, customer data, and secrets stay in the
   customer cloud.

4. **What flows each way:**
   - **Up (satellite → CP):** heartbeat/health, deployed version + config drift, preflight
     report, aggregated metrics (SLOs, not raw).
   - **Down (CP → satellite):** desired Workload spec (image tag, replicas, rollout
     strategy), release-catalog updates, config. The agent **pulls** these; the operator
     reconciles them locally. The CP never pushes directly to the cluster API.

5. **Degraded mode.** Link down → satellite continues serving and reconciling last-known
   desired state; it buffers heartbeats and resumes sync on reconnect. The control plane
   marks the satellite `stale`, not `down`.

### How it threads through the system

- **`connect-agent`** ships in `charts/workload-operator` as an **optional** sub-component;
  it can be disabled for fully air-gapped customers who want only the local operator + GitOps.
- **Preflight** gains an outbound-reachability check to the control-plane FQDN over the
  allowed egress path.
- **Shared-responsibility contract** ([`spec.md`](./spec.md) §4) carries a control-plane
  connectivity row: our component, customer allowlists one FQDN.
- **Observability** is **local-first** — full fidelity stays in the customer cloud; only
  aggregates are forwarded, reinforcing the financial-data boundary.
- **Marketplace/packaging** (future enhancement): a satellite is the unit a customer installs
  and the control plane enrolls — the basis for per-cloud marketplace listings.

---

## 5. Entry Points (Layer 4)

### 5.1 `_agnostic-deploy` — BYOC fast path (PRIMARY)

- **Inputs:** cluster credentials/kubeconfig + a small value set (image tag, resolved key
  reference, ingress class, namespace, optional control-plane enrollment token).
- **Flow (one `terraform apply`):** `preflight (stages 0–5) → k8s-platform installs the
  operator Helm chart via helm_release (incl. connect-agent) → k8s-security +
  k8s-observability → apply Workload CR`.
- **Cloud-agnostic:** pure Kubernetes; needs only cluster access, not full cloud-admin creds.
- **Outcome:** one `terraform apply` produces a secure, observable, lifecycle-managed
  satellite on the customer's existing GKE/EKS/AKS cluster, enrolled with the control plane.
- **Walkthrough:** [`operations/common/verify-on-kind.md`](./operations/common/verify-on-kind.md)
  (generalizes to any existing EKS/GKE/AKS).

### 5.2 `<cloud>-full` — greenfield fallback (SECONDARY)

- **Flow:** `[project (GCP)] → network → kms → cluster → (resolvers) → iam → secrets →
  identical Layer 3 deploy`. Same building blocks; adds provisioning ahead of the identical
  agnostic deploy. Each Layer-1/2 block is independently provision-or-BYO.
- **Single apply.** Greenfield is one `terraform apply`: the in-cluster providers
  (`kubernetes`/`helm`/`kubectl`) read the cluster-resolver's computed endpoint/CA, so
  Terraform defers the in-cluster resources until after the cluster exists, within the same
  apply.
- Preflight runs the same checks, but stages it satisfies *by provisioning* are
  informational rather than blocking. (See [`design.md`](./design.md) §3.)
- **Per-cloud runbooks:** AWS [`operations/aws/deploy.md`](./operations/aws/deploy.md) ·
  GCP [`operations/gcp/deploy.md`](./operations/gcp/deploy.md) ·
  Azure [`operations/azure/deploy.md`](./operations/azure/deploy.md). The per-cloud
  building-block detail is in [`architecture/aws.md`](./architecture/aws.md),
  [`architecture/gcp.md`](./architecture/gcp.md), and [`architecture/azure.md`](./architecture/azure.md).

---

## 6. Repository Structure

```
modules/
  # ── Layer 1 + 2: per-cloud building blocks & platform ──
  # All three clouds share an identical module shape; only the underlying
  # resources differ. The resolvers emit the same output interface in every cloud.
  aws/
    network/            # VPC, subnets, NAT, route tables, AWS Network Firewall (egress)
    network-resolver/   # uniform {vpc_id, subnet_ids, egress_path_ref} (created or looked up)
    iam/                # IRSA roles, workload-identity bindings, rendered policy docs
    kms/                # KMS CMK create-or-BYO
    secrets/            # Secrets Manager + CSI wiring, CMK-encrypted
    cluster/            # EKS, private nodes, hardened
    cluster-resolver/   # uniform {endpoint, ca, auth} (created or looked up)
    preflight/          # per-stage checks owned by this cloud's modules
  gcp/
    project/            # dedicated-project create-or-BYO + required service-API enablement
    network/            # VPC, subnets, Cloud NAT, routes, Cloud NGFW (egress)
    network-resolver/   # uniform {vpc_id, subnet_ids, egress_path_ref} (created or looked up)
    iam/                # GCP service accounts, Workload Identity bindings, custom roles
    kms/                # Cloud KMS CryptoKey create-or-BYO
    secrets/            # Secret Manager + CSI wiring, CMK-encrypted
    cluster/            # GKE (Dataplane V2 = Cilium), private nodes, hardened
    cluster-resolver/   # uniform {endpoint, ca, auth} (created or looked up)
    preflight/          # per-stage checks owned by this cloud's modules
  azure/
    network/            # VNet, subnets, NAT Gateway, route tables, Azure Firewall (egress)
    network-resolver/   # uniform {vpc_id, subnet_ids, egress_path_ref} (created or looked up)
    iam/                # Azure AD workload identity, custom role defs + assignments
    kms/                # Key Vault key create-or-BYO
    secrets/            # Key Vault + CSI wiring, CMK-encrypted
    cluster/            # AKS, private cluster, hardened
    cluster-resolver/   # uniform {endpoint, ca, auth} (created or looked up)
    preflight/          # per-stage checks owned by this cloud's modules

  # ── Layer 3: cloud-agnostic (Terraform wrappers, used by EVERY path) ──
  k8s-platform/         # TF: install model (Tier A operator via helm_release / Tier B manifests)
  k8s-security/         # TF: default-deny NetworkPolicy, namespaced PodSecurity
  k8s-observability/    # TF: Prometheus/metrics, structured logs, dashboards, flow logs
  workload/             # TF: renders the shared `workload` chart (Tier B) / Workload CR (Tier A)

  # ── orchestration ──
  preflight/            # TF module: invokes the checker binary (external data source),
                        #   gates apply via precondition/check, emits green/amber/red report

# Layer 4 composition = customer-facing entry points. These are CONSUMER-OWNED
# composition roots (not shipped product code): reference compositions are
# documented in docs/operations and copied into the consumer's own IaC repo.
#   _agnostic-deploy   ★ BYOC fast path — Layer 3 + preflight, any existing cluster
#   <cloud>-full       greenfield: network → kms → iam → cluster → Layer 3
# BYO permutations (network/cluster/key brought in, rest provisioned) compose the
# same modules with resolvers in lookup mode — no separate module variants needed.

operator/               # Go package tree (Kubebuilder/controller-runtime) — NOT a separate
                        #   module. The single Go module is rooted at the REPOSITORY ROOT
                        #   (module github.com/ops-dev/multicloud-workload-deploy; go.mod at repo
                        #   root) so `//go:embed all:charts/workload` can reach repo-root charts/.
                        #   `operator/` is a package subtree; there is no go.mod under it.
  api/v1/               # Workload CRD types (Spec + Status)
  internal/controller/  # Reconcile loop (renders the shared `workload` chart, adds lifecycle)
  internal/render/      # shared chart-render package (single source of child objects)
  internal/chartfs/     # go:embed of charts/workload (resolves from the repo-root module)
  internal/connect/     # connect-agent: outbound mTLS client, pull loop, heartbeat
  internal/cloud/       # cloud clients (AWS/GCP/Azure) shared by operator + preflight checker
  cmd/preflight/        # preflight checker binary — runs staged checks, emits structured JSON
  config/               # generated CRD, RBAC, manager manifests

charts/
  workload-operator/    # ★ Helm chart: operator (CRD, controller Deployment, RBAC,
                        #   ServiceMonitor, optional connect-agent). Tier A install.
  workload/             # ★ Helm chart: the workload child objects (Deployment, Service,
                        #   HPA, PDB, NetworkPolicy) — single source rendered by BOTH the
                        #   operator (Tier A) and Terraform (Tier B). One values.schema.json.

release/                # Layer 5: BOM template + (generated) per-release bom-<version>.yaml.
                        #   `mage release:*` builds/signs the artifacts and assembles the BOM.
marketplace/            # Layer 5 (future enhancement): per-cloud listing + entitlement artifacts
  aws/                  # AWS Marketplace listing + entitlement wiring
  gcp/                  # GCP Marketplace listing + entitlement wiring
  azure/                # Azure Marketplace listing + entitlement wiring

docs/                   # spec, architecture, design, runbooks, SE playbook
```

---

## 7. Packaging & Distribution (Layer 5)

The product is delivered as a **single BOM-versioned release** — see [`spec.md`](./spec.md) §3 for
the requirement and [`design.md`](./design.md) §6 for the mechanics. The shape:

- **Three artifacts, one version.** A release `vX.Y.Z` pins the operator **OCI image** (by digest),
  the two **Helm charts** (OCI), and the **Terraform module set** (git tag). The chart `appVersion`,
  image tag, and module tag move in lockstep; the chart honors `image.digest` for immutable pins.
- **Provenance.** Image + charts are **cosign-signed** and shipped with an **SBOM** (SPDX), so the
  bundle is verifiable and marketplace-grade.
- **The BOM is cloud-neutral.** Per-cloud marketplace listings (a future enhancement) are a publish
  step that mirrors the BOM's pinned digests into each cloud's registry and adds the cloud's
  entitlement/metering call — they consume the BOM, they don't replace it.
