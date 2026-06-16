# Deploy on GCP (`gcp-full` greenfield)

End-to-end: provision a complete, secure satellite on GCP from nothing — a dedicated project, a
hardened private GKE cluster with a controlled network, encryption, identity, and the
cloud-agnostic Layer-3 deploy on top — then verify and operate it.

> **Already have a cluster?** Use the BYOC fast path instead — a single `terraform apply` against
> the existing cluster. See [`../common/verify-on-kind.md`](../common/verify-on-kind.md) (it
> generalizes to any existing EKS/GKE/AKS). This page is the **greenfield** path that provisions
> the infrastructure too.

Architecture: [`../../architecture/gcp.md`](../../architecture/gcp.md) · Preflight gate:
[`../common/preflight.md`](../common/preflight.md) · Operator/workload ops:
[`../common/workload-operator.md`](../common/workload-operator.md).

---

## 1. What it provisions

```
phase1-infra:  project → network → kms → cluster (Dataplane V2 = Cilium native) →
               iam → kubeconfig
phase2-deploy: secrets → preflight (full mode, GCP provider) →
               Layer-3 (operator, security, observability, workload)
```

- **project** — a dedicated GCP project (or a resolved BYO project) with the required service APIs enabled.
- **network** — VPC + subnet with secondary alias-IP ranges (pods/services), Cloud Router + Cloud NAT,
  a VPC firewall policy (FQDN/CIDR allowlist, default-deny egress), and VPC Flow Logs to a
  retention-locked Cloud Storage bucket.
- **kms** — a customer-managed CryptoKey (rotation on) used for envelope encryption.
- **cluster** — hardened private GKE: Dataplane V2 (Cilium native), Workload Identity, shielded
  nodes, metadata concealment, CMEK database encryption, release channel, control-plane logging.
- **secrets** — Secret Manager secrets (CMEK-encrypted) + the Secrets Store CSI class.
- **iam** — GSA + Workload Identity binding + wildcard-free runtime/deploy custom roles as reviewable artifacts.

There is **no separate Cilium install**: GKE Dataplane V2 is Cilium, and Hubble-grade observability
is enabled on the cluster.

---

> The shipped greenfield entry point is [`roots/gcp-full`](../../../roots/gcp-full). It uses local
> state by default for reviewability; production installs should copy the roots or add a backend
> block that matches the customer's state policy.

## 2. Prerequisites

- `terraform >= 1.7`, the `gcloud` CLI, the `gke-gcloud-auth-plugin`, `kubectl`, `helm`, `go` (to
  build the preflight binary).
- Application Default Credentials (`gcloud auth application-default login`) for an identity holding
  the deploy-time permissions. The `iam` module renders that permission set to
  `modules/gcp/iam/artifacts/deploy-policy/role.json` **during `terraform apply`** (the file is
  generated, not checked in). Provisioning GKE + Cloud NAT incurs real cost.
- A billing account id (when creating a dedicated project) and, optionally, an org or folder id.
- The operator image is distributed **privately**; supply pull credentials so the root creates a
  docker-registry secret and wires it onto the operator ServiceAccount.

```bash
mage preflightBuild   # builds operator/bin/preflight
# Seed terraform.tfvars from the example and edit the placeholders:
cp roots/gcp-full/phase1-infra/terraform.tfvars.example roots/gcp-full/phase1-infra/terraform.tfvars
cp roots/gcp-full/phase2-deploy/terraform.tfvars.example roots/gcp-full/phase2-deploy/terraform.tfvars
```

The operations example [`examples/greenfield.tfvars.example`](./examples/greenfield.tfvars.example)
contains the same workload values in a single file for comparison.

---

## 3. Two-phase apply

`gcp-full` is intentionally split into two Terraform roots. Phase 1 provisions cloud resources and
writes the kubeconfig. Phase 2 reads phase-1 state, runs preflight against the live GKE cluster,
creates secret material, and deploys the cloud-agnostic Layer 3.

```bash
# phase 1: cloud infrastructure
cd roots/gcp-full/phase1-infra
terraform init
terraform apply -auto-approve

# phase 2: preflight + Kubernetes deploy
cd ../phase2-deploy
terraform init

# The refs the preflight binary's --cloud=gcp provider reads from env.
export PREFLIGHT_GCP_PROJECT_ID="$(terraform -chdir=../phase1-infra output -raw project_id)"
export PREFLIGHT_GCP_REGION="$(terraform -chdir=../phase1-infra output -raw region)"
export PREFLIGHT_GCP_ROUTER_NAME="$(terraform -chdir=../phase1-infra output -raw router_name)"

terraform apply -auto-approve
```

Expected: `terraform output preflight_verdict` is `green` or `amber` (full mode downgrades
provisioned cloud stages to informational); `terraform output install_tier` is `A`.

> The GKE node pool needs Cloud DNS private zones (created by the `network` module) resolving
> `googleapis.com` / `gcr.io` / `pkg.dev` to the restricted Private Google Access VIP — without
> them, nodes under default-deny egress cannot reach Artifact Registry and never go Ready. The
> module provisions these automatically.

---

## 4. Operating notes (GCP-specific)

- **Private cluster access.** The control-plane endpoint is private by default, so `kubectl` works
  from **inside the VPC**. For a one-off check from outside, flip the endpoint public and scope it
  to your IP — do **not** commit the IP — then flip it back:
  ```bash
  terraform apply -auto-approve \
    -var enable_private_endpoint=false \
    -var "master_authorized_networks=[{cidr_block=\"$(curl -s https://checkip.amazonaws.com)/32\",display_name=\"test\"}]"
  ```
- **Egress is default-deny.** Only allowlisted FQDNs/CIDRs leave the VPC; add to
  `egress_allowed_fqdns` / `egress_allowed_cidrs` and re-apply. Dataplane V2's `toFQDNs` is the
  in-cluster second layer.
- **Private operator image.** Rotate the pull-secret credentials by updating
  `operator_image_pull_secret` and re-applying.
- **Reviewable least-privilege artifacts.** `terraform apply` renders the runtime + deploy-time
  policies under `modules/gcp/iam/artifacts/` (generated, not checked in) — enumerated permissions,
  no primitive roles, no wildcards — for inspection before granting. The committed source of truth
  is the `iam` module and its `tests/no_wildcards.tftest.hcl` golden test.

Workload lifecycle, the preflight gate, and post-deploy verification are covered by the common
runbooks: [`../common/workload-operator.md`](../common/workload-operator.md) and
[`../common/preflight.md`](../common/preflight.md).

---

## 5. BYO variations (deploy into customer-owned infra)

The pieces — `project_mode` / `network_mode` / `kms_mode` / `iam_mode` / `cluster_mode` — are
**independent** toggles (`provision` | `byo`). The preflight cloud stages self-gate: a BYO concern
reports **amber (not applicable)**, never red. Each scenario below has a complete, copy-pasteable
example in [`examples/`](./examples).

| Scenario | What you bring / we provision | Example |
|---|---|---|
| **BYO project** | Customer's project; we provision network + KMS + GKE + secrets + identity | [`byo-project.tfvars.example`](./examples/byo-project.tfvars.example) |
| **BYO project + VPC + KMS** | Customer's project, network, CryptoKey; we provision the cluster + identity | [`byo-network-kms.tfvars.example`](./examples/byo-network-kms.tfvars.example) |
| **BYO everything** | Customer's project, network, KMS, identity, cluster; we only deploy Layer 3 | [`byo-everything.tfvars.example`](./examples/byo-everything.tfvars.example) |

Key snippets:

**BYO project** — resolve an existing project (and ensure the required APIs), provision the rest:

```hcl
project_mode = "byo"
project_id   = "customer-project-id"
```

**BYO VPC** — the customer's subnet must carry secondary ranges named `<name>-pods` /
`<name>-services`. There is no retention-locked flow-log bucket (customer owns logging);
`egress_path_ref` is empty (customer owns the edge):

```hcl
network_mode     = "byo"
byo_network_name = "customer-vpc"
byo_subnet_name  = "customer-subnet"
```

**BYO KMS** — resolve a customer CryptoKey by its canonical resource id:

```hcl
kms_mode       = "byo"
byo_kms_key_id = "projects/P/locations/L/keyRings/R/cryptoKeys/K"
```

**BYO cluster / everything** — set the remaining modes to `byo` and supply the `byo_*` values.
Every cloud stage reports amber; the deploy reduces to the Kubernetes stages. Prefer the BYOC fast
path (a single apply, no cloud provider) unless you specifically need the `gcp-full` composition.

---

## 6. Teardown (the flow-log bucket does NOT auto-destroy; the CryptoKey is protected)

Two resources resist a plain `terraform destroy`:

- The **flow-log bucket** has a locked retention policy: objects cannot be deleted before retention
  elapses. This is intentional (the retention-locked audit floor).
- The **CryptoKey** has `prevent_destroy = true` (KMS keys cannot be truly deleted, only scheduled
  for destruction).

```bash
# 1. Destroy phase 2 first (Layer 3 + secrets).
cd roots/gcp-full/phase2-deploy
terraform destroy -auto-approve

# 2. Destroy phase 1 EXCEPT the flow-log bucket and the CryptoKey.
cd ../phase1-infra
terraform destroy -auto-approve \
  -target=module.iam -target=module.cluster

# 3. CryptoKey: remove the prevent_destroy lifecycle block and re-apply before destroying it,
#    or leave the key in place (KMS keys are scheduled for destruction, not deleted immediately).
# 4. Flow-log bucket: empty all object versions and destroy module.network once retention elapses;
#    until then the objects cannot be deleted.
```

The `TestGCPFullGreenfield` e2e automates this whole flow against a real project.
