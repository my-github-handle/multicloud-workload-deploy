# Running Preflight

The preflight checker validates deploy prerequisites bottom-up and emits a green/amber/red report.
The Terraform deploy paths run it automatically and gate `apply` on the verdict; you can also run
it standalone to diagnose a cluster before deploying.

> Project guide: [`README.md`](../README.md) · Design:
> [`../components/preflight-checker.md`](../../components/preflight-checker.md).

---

## Build

```bash
mage preflightBuild        # → operator/bin/preflight
```

## Run standalone against a cluster

```bash
operator/bin/preflight \
  --kubeconfig "$KUBECONFIG" \
  --namespace <app-namespace> \
  --mode agnostic \
  --exit-on-red          # optional: exit non-zero on a red verdict (CLI convenience)
```

Pretty-print the staged report:

```bash
operator/bin/preflight --kubeconfig "$KUBECONFIG" --namespace <ns> \
  | jq -r '.report_json | fromjson | .stages[] | "\(.id) \(.name): \(.status)"'
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `--kubeconfig` | "" | path to the target cluster's kubeconfig; empty on a real run yields a **blocking red Stage 4** (no cluster to deploy to) |
| `--namespace` | `default` | target workload namespace (Stage 4/5 checks) |
| `--mode` | `agnostic` | `agnostic` (BYOC — every stage blocks) or `full` (greenfield — cloud stages 0–3 are informational) |
| `--cloud` | "" | `aws` \| `gcp` \| `azure`; empty uses the built-in fake (cloud stages report stub green). Real providers are added per cloud. |
| `--exit-on-red` | false | exit non-zero on a red verdict; for standalone CLI use only |

---

## Reading the result

Output is a flat JSON object with two string keys:

```json
{ "verdict": "green|amber|red", "report_json": "<full staged Report as a JSON string>" }
```

- **green** — all prerequisites met; apply proceeds.
- **amber** — documented gaps (e.g. no Cilium, no metrics-server, Tier B install); apply proceeds,
  gaps recorded.
- **red** — a blocking failure; apply must stop. The offending `CheckResult` carries a `message`
  and `remediation`.

The verdict is computed as: **red if any blocking stage is red; else amber if any blocking stage
is amber; else green.** A non-blocking stage (the cloud stages 0–3 in greenfield `full` mode, whose
resources are created by phase 1) keeps its true severity in the report but contributes at most
amber to the verdict — it never makes the verdict red. The first blocking red short-circuits the
rest (later stages are marked `skipped`).

The binary **always exits 0** when there is no `--exit-on-red`, even on internal errors (bad
kubeconfig, unimplemented `--cloud`) — those become a red verdict carrying the error, so a caller
(Terraform) gates on the verdict rather than a crash.

### Red conditions (what blocks a deploy)

In the BYOC (`agnostic`) path these are the Kubernetes-stage reds — all check the real cluster:

| Result ID | Red when |
|---|---|
| `k8s.unreachable` | no kubeconfig given on a real run; the target cluster is not reachable |
| `k8s.networkpolicy` | the `networking.k8s.io` (NetworkPolicy) API is not served, or API groups cannot be listed |
| `k8s.minversion` | the cluster Kubernetes version is below the supported floor, or cannot be read |
| `k8s.installtier` | the deploy identity cannot create namespaced Deployments (neither Tier A nor Tier B is possible), or a SelfSubjectAccessReview call fails |
| `k8s.workloadidentity` | the workload ServiceAccount does not exist and cannot be created |
| `workload.namespace` | the target namespace does not exist and cannot be created (or cannot be read) |

The cloud stages (0–3) add these reds once the per-cloud providers are in use (greenfield marks
them non-blocking, so they surface as amber there):

| Result ID | Red when |
|---|---|
| `iam.missing` | the deploy identity is missing a required permission |
| `kms.key` / `kms.permissions` | the resolved key is missing/disabled, or the identity cannot use it |
| `secrets.*` | the secrets backend is unreachable or material is not CMK-encrypted |
| `egress.controlplane_fqdn` | the control-plane FQDN is unreachable — the connect-agent cannot dial home |
| `egress.metadata_block` / `egress.ghcr` / `egress.cloud_api` | a required egress path over the allowed route fails |

Everything else (no Cilium, no metrics-server, Tier B install, Argo absent/incompatible,
image-pull not verified) is **amber**, not red — see the table below.

### Can a deploy proceed when preflight fails?

- **amber → yes.** Amber means "documented gaps"; the apply proceeds and the gaps are recorded in
  the report. This is the intended path for clusters that are usable but not fully featured.
- **red → no, by default.** The Terraform `preflight` module hard-gates a red verdict
  (`fail_on_red = true`, the default) on the data source's postcondition, so the plan fails before
  any resource is created.
- **Overriding a red.** Setting `fail_on_red = false` disables the hard block. Even then, the most
  dangerous case is still caught: if red is because the identity cannot deploy at all
  (`k8s.installtier` red), `install_tier` is derived as `"RED"`, which is neither `"A"` nor `"B"`
  and trips the downstream module validation — so the apply fails loudly rather than half-deploying
  something the cluster provably cannot run. Override a red only when you understand the specific
  failure and have a reason to proceed (e.g. a known-false-positive in a custom environment).

---

## How Terraform uses it

The Terraform deploy path invokes the binary through an `external` data source and blocks `apply`
on a red verdict (illustrative — the Terraform module ships with the Layer-3 deploy path):

```hcl
data "external" "preflight" {
  program = [local.preflight_binary, "--mode=agnostic",
             "--kubeconfig", var.kubeconfig, "--namespace", var.namespace]
}

check "preflight_not_red" {
  assert {
    condition     = data.external.preflight.result.verdict != "red"
    error_message = "Preflight failed (red): ${data.external.preflight.result.report_json}"
  }
}
```

Because the binary exits 0 and prints a flat string map, the data source never errors; the gate
keys on `verdict`.

---

## Common results & remediation

| Result ID | Meaning when not green | Action |
|---|---|---|
| `k8s.unreachable` (red) | no kubeconfig / cluster unreachable | pass `--kubeconfig` for the target cluster |
| `k8s.networkpolicy` (red) | NetworkPolicy API not served | install a CNI that supports NetworkPolicy |
| `k8s.minversion` (red) | cluster older than the supported floor | upgrade the cluster to the minimum supported Kubernetes version |
| `k8s.installtier` (amber) | Tier B — namespace-only permissions | operator lifecycle unavailable; grant CRD+ClusterRole create for Tier A, or use [Tier B](./helm-only-tier-b.md) |
| `k8s.installtier` (red) | cannot create namespaced Deployments | grant `create` on `apps/deployments` in the namespace |
| `k8s.metricsserver` (amber) | metrics-server absent | install metrics-server so the HPA can scale on CPU |
| `k8s.cilium` (amber) | Cilium not detected | FQDN egress/Hubble fall back to the perimeter firewall + cloud flow logs |
| `k8s.argorollouts*` (amber) | Argo Rollouts absent/incompatible/no traffic primitive | canary degrades to RollingUpdate / replica-weighted |
| `k8s.workloadidentity` (red) | workload SA absent and not creatable | grant `create` on serviceaccounts, or pre-create the annotated SA |
| `egress.controlplane_fqdn` (red) | control-plane FQDN unreachable | allow the connect-agent's outbound FQDN through the egress firewall |
| `workload.namespace` (red) | namespace missing and not creatable | pre-create the namespace, or grant `create` on namespaces |

---

## Modes: agnostic vs full

- **agnostic** (BYOC, the primary path): every stage blocks. A red anywhere stops the apply and
  short-circuits later stages.
- **full** (greenfield `<cloud>-full/phase2-deploy`): cloud stages (0–3) cover resources created
  by phase 1, so they're informational — they still run and report their true severity, but a red
  there does not block and does not skip the Kubernetes stages. A red in a Kubernetes stage still
  blocks.

---

## Cloud-specific stages and when they apply

Stages 0–3 are the **cloud-facing** stages, run by the per-cloud provider selected with `--cloud`
(e.g. `--cloud=aws`); with no `--cloud` a fake provider runs and these stages are inert. Stages 4–5
are **cloud-agnostic** and always run against the target cluster.

| Stage | ID | Checks | Applies when |
|---|---|---|---|
| Identity | 0 | deploy identity can **provision** the in-scope concerns | something is being provisioned |
| KMS | 1 | resolved CMK exists, enabled, rotating | a CMK is configured (we envelope-encrypt) |
| Secrets | 2 | secret is CMK-encrypted | secrets are configured |
| Egress/network | 3 | VPC available, NAT, firewall-in-path, metadata-block, FQDN reachability | a VPC is configured |
| Kubernetes | 4 | cluster reachable, install tier, NetworkPolicy/PSA, metrics-server, CNI | **always** (the deploy target) |
| Workload | 5 | namespace, ServiceAccount/identity binding, workload readiness | **always** |

**A cloud stage is not-applicable, not a failure, when its concern is BYO.** Each cloud stage
self-gates: if the thing it would check isn't in play, it reports **amber (informational)** rather
than red.

- **Identity (0)** simulates only the create-actions for the concerns being **provisioned**. If
  every concern is BYO (nothing provisioned), it reports amber — there's no provisioning
  permission to require. Scope is passed via `PREFLIGHT_AWS_PROVISION_CONCERNS` (comma-separated
  subset of `kms,secrets,iam,cluster`; empty = all = greenfield).
- **KMS (1)** reports amber when no CMK ARN is configured (nothing is encrypted by us).
- **Secrets (2)** reports amber when no secrets are configured.
- **Egress (3)** reports amber for `egress.vpc` when no VPC is configured, and `egress.firewall_inpath`
  is amber when the customer owns the edge (no `egress_path_ref`) — a shared-responsibility item we
  cannot assert.

### What this means for a BYO cluster

When the customer **brings the cluster** (and possibly the network/key/identity too), the
load-bearing checks shift from "can we provision cloud infra?" to "can we deploy onto what
exists?":

- The **BYOC fast path (`roots/agnostic-deploy`)** runs **no** `--cloud` provider at all — stages
  0–3 are inert, and the gate is entirely **stages 4–5** plus the shared-responsibility contract.
- The greenfield `roots/<cloud>-full/phase2-deploy` path with **BYO toggles** runs the cloud
  provider but the cloud stages degrade to amber per BYO concern, so the report shows exactly which
  infra we own vs. the customer.
- The one cloud check that still matters in BYO is **egress reachability to the control-plane
  FQDN** — the customer's edge must permit it. We surface it (amber, shared responsibility) but
  cannot enforce it.
