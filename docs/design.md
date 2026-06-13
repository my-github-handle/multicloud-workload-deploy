# BYOC Multi-Cloud Workload Deployment — Detailed Design

**Owner:** Infrastructure / Platform
**Status:** Draft for review
**Version:** 1.0

> Companion documents: [`spec.md`](./spec.md) (requirements & scope) ·
> [`architecture.md`](./architecture.md) (system shape, layering, satellite model,
> entry points, repository structure).

This document covers the detailed engineering design: module contracts, the operator and
its workload lifecycle, the layered preflight, rollout strategy, and the testing strategy.

---

## 1. Layer-1 Module Contracts

Each module supports **provision-or-BYO** and exposes a stable output interface. The
create-vs-lookup branch is isolated in the corresponding resolver
(see [`architecture.md`](./architecture.md) — Core Conventions).

### 1.1 `network` (+ egress/firewall)

- **Provision:** private VPC/subnets, no public IPs on nodes, NAT for controlled egress,
  **egress firewall with FQDN/CIDR allowlist** (AWS Network Firewall / GCP Cloud NGFW /
  Azure Firewall), default-deny everything else (incl. the one control-plane FQDN).
- **BYO:** consume existing VPC/subnet IDs; apply what we can (in-cluster policy templates),
  document edge-firewall gaps as customer responsibility.
- **Outputs:** `vpc_id`, `subnet_ids`, `egress_path_ref`.

### 1.2 `iam` (workload identity + least-privilege policy)

- Workload runtime identity via **IRSA (AWS) / GKE Workload Identity / AKS Workload
  Identity** — no static keys. Binds the workload + connect-agent ServiceAccount to a cloud
  identity.
- **Outputs:** `workload_identity_ref`, `role_arn/sa_email/client_id`, plus the rendered
  policy documents (for customer review / BYO attachment).

#### Least-privilege policy model

The `iam` module produces concrete policy documents for two distinct identities, each scoped
to the minimum it needs:

| Identity | Used by | Permission set (example) |
|---|---|---|
| **Deploy-time identity** | the `terraform apply` operator (SE or CI) | create/manage only the resources in the path being run — scoped to the target VPC/cluster/key/secret resources, no account-wide wildcards |
| **Runtime workload identity** | the workload + connect-agent pods | `kms: Decrypt/GenerateDataKey` on **the resolved key only**; `secrets: GetSecretValue` on **the workload's secret paths only**; pull from the image registry; write logs/metrics to the designated sink |

Design rules:

- **Derived from the action set.** Each module declares the exact API actions it performs; the
  `iam` module composes those into a policy. Adding a capability to a module updates its
  declared actions, which regenerates the policy — the policy can't silently drift from what
  the code does.
- **Resource-scoped, no wildcards.** Policies name specific ARNs / resource IDs / key
  references (the resolver outputs), never `Resource: "*"` or `kms:*`. Conditions pin region,
  account, and (where supported) the calling workload identity / OIDC subject.
- **Per-cloud, equivalent intent.** AWS IAM policy JSON + condition keys, GCP custom roles +
  IAM bindings (with Workload Identity `principal://` members), Azure custom role definitions
  + role assignments — three renderings of the same least-privilege intent.
- **Shipped as reviewable artifacts.** The rendered policy documents are emitted as module
  outputs / files so a customer can inspect (and, in BYO-identity mode, attach) them before
  granting anything.
- **BYO-identity mode.** If a customer prefers to create the role themselves, the module emits
  the exact policy + trust/assume document for them to attach, and resolves the supplied
  identity — same resolver pattern as network/cluster/key.
- **Asserted by preflight.** Stage 0 checks the deploy identity holds exactly the needed
  permissions (and flags both *missing* and *excess* where detectable); Stage 4 confirms the
  runtime Workload Identity binding resolves end-to-end. (See §3.)

### 1.3 `kms` (CMK / BYO key)

- **Provision:** customer-managed key (AWS KMS CMK / GCP Cloud KMS CryptoKey / Azure Key
  Vault key) with rotation policy.
- **BYO:** resolve a customer-supplied key ARN/resource-id; verify enabled + usable.
- **Consumed by:** `secrets` (envelope encryption), `cluster` (secrets/disk encryption at
  rest), any persistent volumes.
- **Outputs:** `key_id` / `key_arn` / `key_resource_id`.

### 1.4 `secrets`

- Backend wiring (Secrets Manager / Secret Manager / Key Vault) + CSI driver or sync
  mechanism. Secret material **envelope-encrypted with the resolved CMK** (Stage-1 key).
- **Outputs:** `secrets_ref`, mounting/sync config for the workload.

---

## 2. Layer 3 — Cloud-Agnostic Kubernetes Layer

### 2.1 Operator (`operator/`)

- **Build:** Go, Kubebuilder / Operator SDK on `controller-runtime`.
- **Packaging & install:** the operator ships as `charts/workload-operator` (CRD, controller
  Deployment, RBAC, ServiceMonitor, optional connect-agent). The workload's child objects ship
  separately as `charts/workload` (see §2.5), so the same child templates are rendered by both
  the operator (Tier A) and Terraform (Tier B). Terraform installs the operator chart via the
  `helm` provider (`helm_release` in `k8s-platform`); both charts are standalone
  `helm install`-able for GitOps/manual use. Chart versions are pinned per release.
- **CRD:** one `kind: Workload` (`api/v1`). Spec covers image/tag, replicas/autoscale
  bounds, probes, rollout strategy, key reference, ingress class, resource requests/limits.
  Status carries conditions, observed rollout state, and events.
- **Reconciles:** Deployment (the workload image), Service, **HPA** (autoscaling),
  **PDB**, probes (liveness/readiness/startup). See §4 for rollout strategy.
- **Observability (first-class):** `controller-runtime` Prometheus metrics endpoint
  (reconcile counts/latency/errors) + domain metrics + rich status conditions + Kubernetes
  events.
- **Cloud-agnostic:** pure Kubernetes API. Cloud differences (storage class, ingress class,
  IAM annotations) arrive via the Workload spec / per-cloud values injection.
- **Scope discipline:** one workload shape; no generic multi-container orchestration.

### 2.2 `k8s-security`

- **Default-deny `NetworkPolicy`** for the workload namespace: pods cannot reach each other,
  the control plane, or the **cloud metadata endpoint (`169.254.169.254`)** — a primary
  credential-theft vector. Explicit allowlist for required egress only (incl. the
  control-plane FQDN for the connect-agent).
- **Namespaced PodSecurity** (restricted): non-root, no privilege escalation, seccomp,
  dropped capabilities.
- **Resource quotas** bounding blast radius.
- **Hardening tier (clearly marked, optional):** strong runtime isolation (gVisor / Kata or
  per-tenant node pools with taints) for untrusted-code isolation. Core deliverable =
  default-deny network + metadata block + PodSecurity + flow logs + audit logging.

#### CNI strategy

The CNI is a detected capability. In BYO clusters it is cluster-scoped, set at cluster
creation, and owned by the customer, so it is not swapped.

- **Greenfield (`<cloud>-full`): Cilium is provisioned.** It provides, uniformly across
  EKS/GKE/AKS, **FQDN-based egress** (`toFQDNs` for the control-plane FQDN + `ghcr.io` + cloud
  APIs), metadata-IP blocking, L7 visibility, transparent WireGuard encryption, and Hubble
  flow visibility. This aligns with GKE Dataplane V2.
- **BYO: standard Kubernetes `NetworkPolicy` is the portable floor.** Preflight Stage 4
  requires NetworkPolicy support (any conformant CNI); default-deny and metadata-block are
  enforced via plain `NetworkPolicy`. Where the cluster runs Cilium, `toFQDNs`/Hubble are
  layered on. Where it does not, FQDN-granular in-cluster egress is an amber gap, covered at
  the perimeter by the FQDN allowlist in the `network` egress firewall.
- The cloud edge egress firewall (`network`, §1.1) is the FQDN backstop that holds regardless
  of CNI; the in-cluster CNI policy is the additional, identity-aware layer.

### 2.3 `k8s-observability`

- **Metrics (Prometheus):** operator reconcile health, workload SLOs, HPA/autoscale signals.
- **Logs (structured, cloud-native sink):** include audit-relevant events — denied egress,
  policy violations, secret access.
- **Network flow visibility — two layers (audit floor + detection enhancement):**
  - **Cloud VPC flow logs = always-on audit floor.** Enabled in every path (greenfield and
    BYO), shipped to a **customer-owned, retention-locked** sink (S3 / Cloud Logging /
    Storage). CNI-independent and **survives cluster compromise** — this is the immutable
    audit trail of record for financial data. The `network` module owns enabling it
    (greenfield) or asserting/requesting it (BYO).
  - **Hubble = Cilium-gated detection enhancement.** Where Cilium runs (greenfield, or
    BYO-with-Cilium detected by Preflight), Hubble adds pod/namespace/label identity, L7
    HTTP, DNS, and policy-verdict (ALLOWED/DENIED) visibility — the actionable
    exfil-detection and triage layer. Where Cilium is absent, Preflight marks Hubble-grade
    visibility as an **amber gap**; the cloud flow-log floor still satisfies the audit
    requirement.
  - The two layers observe different vantage points: Hubble is high-signal but lives inside
    the cluster, while cloud flow logs are independent of it, durable, and customer-owned.
    Both are present so the audit trail does not depend on the integrity of the data plane.
- **Local-first.** Full-fidelity telemetry (both layers) stays in the customer cloud; only
  aggregated/redacted signals are forwarded to the control plane
  (see [`architecture.md`](./architecture.md) — Satellite Architecture). Reinforces the
  financial-data boundary.
- **Infra→production feedback loop:** tie Terraform changes to these signals so a config
  change's effect on egress posture, error rate, and rollout health is observable —
  tightening the loop between infra change and production behavior.

### 2.4 `connect-agent` (satellite side)

- Outbound mTLS client living in the workload namespace; opens a persistent connection to the
  control-plane FQDN and runs a **pull loop** for desired-state deltas plus a **heartbeat**.
- Applies pulled desired state by updating the Workload CR; the operator reconciles locally.
- **Degraded mode:** link down → buffer heartbeats, keep last-known desired state; resume sync
  on reconnect. Never blocks local reconciliation.
- Optional/disable-able for air-gapped customers.
- **Transport deferred.** This deliverable fixes the **contract** — outbound-only mTLS, one
  allowlisted FQDN, pull-based desired state, local-first telemetry, degraded-mode behavior —
  but the concrete wire protocol (e.g. gRPC stream vs. reverse tunnel) and the enrollment/cert
  issuance flow are deferred to the control-plane workstream
  ([`spec.md`](./spec.md) §5). The satellite-side interface is designed so the transport can be
  slotted in without changing the operator or the security posture.

### 2.5 Install model & namespace-only fallback

A CRD is a **cluster-scoped** object: installing or upgrading it requires `create`/`update` on
`customresourcedefinitions.apiextensions.k8s.io`, which can only come from a ClusterRole. A
CRD therefore cannot be installed with namespace-only permissions. The install model is a
capability tier, selected by Preflight (§3, Stage 4):

- **Tier A — operator (default).** The CRD and the operator's RBAC are installed once as a
  **cluster-scoped bootstrap** (the customer's cluster admin runs the chart, or grants a
  scoped, time-boxed permission for the CRD + RBAC bootstrap only). The controller then runs
  **namespace-scoped**: its `controller-runtime` cache is restricted to the workload namespace
  (`Cache.DefaultNamespaces`) and its ServiceAccount is bound with a **Role**, not a
  ClusterRole. The CRD is cluster-visible (unavoidable), but the controller reads and writes
  only its own namespace. This preserves the full `Workload` lifecycle: status conditions,
  drift correction, and capability-gated rollout.

- **Tier B — operator-less (namespace-only).** When no cluster-scoped object may be created,
  no CRD or operator is installed. The `workload` Terraform module renders plain **namespaced**
  resources directly via the `kubernetes`/`helm` providers — `Deployment`, `Service`, `HPA`,
  `PDB`, `NetworkPolicy` — into the granted namespace. Lifecycle becomes Terraform-plan-driven
  rather than operator-driven: the `Workload` abstraction, status conditions, drift correction,
  and operator-emitted canary are not available. The workload still runs, stays within the
  security posture, and remains observable (namespaced ServiceMonitor + cloud flow logs).

The operator is an enhancement over the namespaced-manifest floor — the same portable-floor /
detected-enhancement model used for the CNI (§2.2) and rollout strategy (§4).

**Shared rendering.** The workload's child objects (Deployment, Service, HPA, PDB,
NetworkPolicy) are defined once in `charts/workload`, with a single `values.schema.json`. Both
tiers render that one source: in Tier A the operator renders the chart (via
`operator/internal/render`) and then owns the objects, adding status conditions, drift
correction, and rollout; in Tier B Terraform renders the identical chart via `helm_release`.
The `Workload` CR spec and the Terraform variables share the chart's value schema, so the two
tiers cannot drift. What differs between tiers is lifecycle ownership, never the manifests.

---

## 3. Layered Preflight (staged validation gate)

Runs **bottom-up in dependency order**, mirroring the layer graph. Each stage gates the
next; failure stops with an **actionable report scoped to that layer**, so a customer never
debugs a Kubernetes failure that is really a missing IAM permission.

```
Stage 0  Identity & access     ─┐  Layer-1 building-block prereqs
Stage 1  KMS / encryption keys   │  (BYO resources reachable + usable)
Stage 2  Secrets backend         ┘
Stage 3  Network / egress       ── connectivity & egress-posture prereqs
Stage 4  Kubernetes infra       ── cluster capabilities (the deploy target)
Stage 5  Workload readiness     ── final go/no-go for the Workload CR
```

- **Stage 0 — Identity & access.** Caller creds valid; can assume/impersonate deploy
  identity. Deploy identity holds **exactly** the least-privilege permissions the path needs —
  checked against each module's declared action set, flagging both *missing* permissions
  (blocking) and *excess* permissions where detectable (warning). Workload Identity mechanism
  available & enabled.
- **Stage 1 — KMS.** Resolved key exists and is enabled; deploy identity can
  Encrypt/Decrypt/GenerateDataKey; rotation/region/key-policy constraints satisfied.
- **Stage 2 — Secrets.** Backend reachable; CSI/sync mechanism available; secret material is
  CMK-encrypted with the Stage-1 key.
- **Stage 3 — Network / egress.** VPC/subnets resolvable with capacity + routing; private
  nodes / no public IP; controlled egress path (NAT/firewall) present; metadata endpoint
  blockability assessed; connectivity to required endpoints over the allowed path —
  `ghcr.io`, cloud APIs, observability sinks, **and the control-plane FQDN** (for the
  connect-agent's outbound tunnel).
- **Stage 4 — Kubernetes infra (BYO-cluster gate).** Cluster reachable via resolver; min
  K8s version; **CNI with NetworkPolicy support** (the portable floor; else egress
  default-deny unenforceable → flagged gap); **Cilium detection** (present → enable
  `toFQDNs`/Hubble enhancements; absent → FQDN egress + Hubble flagged as amber gaps, covered
  by the perimeter egress firewall + cloud flow logs); metrics-server present (HPA);
  **install-tier selection** (§2.5) from the available permissions — cluster-scoped CRD/RBAC
  create → Tier A (operator); namespace-only → Tier B (operator-less namespaced manifests),
  reported as an amber gap; neither (cannot create namespaced workloads) → red, stop;
  PodSecurity admission available; Workload Identity binding wires SA → cloud identity
  end-to-end; **Argo Rollouts controller/CRD presence + version** and traffic-routing
  primitive detected (drives the §4 rollout outcome).
- **Stage 5 — Workload readiness (go/no-go).** Namespace free/creatable; ingress class
  resolvable; storage class for PVs is CMK-encrypted; image pullable with resolved creds.
  Emits a single **green/amber/red** report: green → apply proceeds; amber → proceed with
  documented gaps; red → stop.

### Modularization

- **Each cloud building block ships its own preflight contract**, co-located with the module
  (`modules/<cloud>/iam/preflight.*`, `.../kms/`, `.../network/`, `.../cluster-resolver/`).
  The module that knows a requirement owns its check.
- A thin **`preflight` orchestrator** sequences the stages and aggregates reports. Both entry
  points use it: `_agnostic-deploy` runs all stages against existing infra (blocking);
  `<cloud>-full` runs the same checks, but stages it satisfies by provisioning are
  informational rather than blocking. **Same checks, different blocking semantics — no
  duplicated logic.**

### Invocation: Terraform-driven, no wrapper script

Terraform is the single driver ([`architecture.md`](./architecture.md) §3); there is no
`deploy.sh`-style orchestrator. Preflight runs as a **tested checker binary invoked from
Terraform**, not a shell script:

- The checks are a Go binary that reuses the operator's cloud clients
  (`operator/internal`) — shared SDK wiring, auth, and types, so preflight and the operator
  never diverge. It is unit-tested (§5), reviewable, and shipped in the release tooling image.
- The `preflight` module invokes it through an `external` data source and surfaces the
  structured result; module `precondition`/`check` blocks gate `terraform apply` on it, so a
  **red** result fails the plan before any resource is created. **Amber** results pass the
  gate and are recorded in the report with their documented gaps.
- The full green/amber/red staged report is emitted as a Terraform output (and a written
  artifact) for the SE to read — the rich reporting Terraform's native checks cannot express
  on their own. The whole flow remains one `terraform apply`, fully reviewable via
  `terraform plan`.

---

## 4. Rollout Strategy & Argo Rollouts (capability-gated)

Rollout strategy is a **detected capability** — the operator delegates traffic-shifting to
Argo Rollouts when the cluster supports it and degrades cleanly otherwise.
Argo Rollouts is itself a **cluster-scoped controller + CRDs** (a second operator), so in BYOC
it falls under the same shared-responsibility model as the cluster
([`spec.md`](./spec.md) §4).

- **Workload spec declares intent:** `rolloutStrategy: RollingUpdate | Canary`.
- **Preflight Stage 4 detects** whether the Argo Rollouts controller/CRDs are present and what
  traffic-routing primitive exists (mesh? ingress class?).
- **Three outcomes:**
  1. **Customer already runs Argo Rollouts** → we **attach**: the operator emits `Rollout`
     objects into our namespace referencing their controller. We do **not** install or upgrade
     their cluster-scoped controller; preflight asserts a compatible CRD version.
  2. **Greenfield / customer permits cluster install** → Layer 3 installs the Argo Rollouts
     Helm chart as an optional sub-release, and canary works fully.
  3. **BYO cluster, no Argo, namespaced-only permissions** → **degrade gracefully** to
     Kubernetes-native `RollingUpdate` (maxSurge/maxUnavailable + readiness gates + PDB).
     Canary is reported as an **amber gap**, not a hard failure.
- **Traffic-shifting also degrades:** without a mesh/compatible ingress, even with the
  controller present, canary falls back to **replica-weighted** canary (no fine-grained
  traffic %). Preflight flags this.
- The default path (`RollingUpdate`) needs zero extra cluster components and works on any
  conformant cluster. Canary is an opt-in enhancement that activates only when the cluster can
  support it.

---

## 5. Module Boundaries & Testing Strategy

- **Isolation:** every module answers *what it does / how to use it / what it depends on*.
  Resolvers keep cross-module interfaces uniform so internals can change without breaking
  consumers.
- **Terraform validation:** `fmt`, `validate`, `tflint`, per-module example fixtures; plan
  checks in CI for each `live/` entry point against mock/test backends.
- **Operator tests:** `envtest` unit tests for the reconcile loop (Deployment/Service/HPA/PDB
  creation, status conditions, requeue behavior); coverage target ≥ 80%.
- **Connect-agent tests:** outbound mTLS handshake against a stub control plane; pull-loop
  applies desired-state deltas; degraded-mode (link-down → local reconcile continues) and
  reconnect/heartbeat-resume behavior.
- **Helm chart tests:** `helm lint` + `helm template` schema/render checks; chart installs
  cleanly into a kind cluster in CI; CRD upgrade compatibility check across chart versions;
  connect-agent enable/disable toggle renders correctly.
- **Preflight tests:** per-stage check units with simulated pass/amber/fail fixtures
  (incl. Argo-present/absent and control-plane-FQDN reachable/blocked); end-to-end
  staged-report assembly test.
- **Security checks:** policy unit tests for default-deny NetworkPolicy and metadata-block;
  PodSecurity admission assertions; assert only the control-plane FQDN is egress-allowed.
- **CNI / observability gating:** assert greenfield provisions Cilium with `toFQDNs` + Hubble;
  assert BYO-without-Cilium falls back to the NetworkPolicy floor with FQDN/Hubble reported as
  amber gaps; assert cloud VPC flow logs are enabled and shipped to the customer-owned sink in
  **every** path (greenfield and BYO), CNI-independent.
- **Install tiers:** assert Tier A renders the namespace-scoped controller (Role, not
  ClusterRole; cache scoped to the namespace) and that the rendered workload objects match
  Tier B's namespaced manifests for the same `Workload` input; assert Tier B renders no
  cluster-scoped object; assert Stage-4 tier selection picks A/B/red from the available
  permissions.
- **Least-privilege IAM:** golden-file tests on the rendered per-cloud policy documents
  (deploy-time + runtime) — assert no wildcards (`*` / `kms:*` / `Resource: "*"`), that
  resources are pinned to resolver outputs, and that the action set matches the modules in the
  path; assert that adding a module action updates the rendered policy (no drift).
