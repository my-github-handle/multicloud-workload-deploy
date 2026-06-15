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
project → network → kms → cluster (Dataplane V2 = Cilium native) →
        (network-resolver, cluster-resolver) → secrets → iam →
        preflight (full mode, GCP provider) →
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

> The **greenfield composition root** (`gcp-full`) is consumer-owned scaffolding, not shipped
> product code — the shipped product is the `modules/gcp/*` building blocks + the charts. Copy the
> reference composition into your own IaC repo, wire your backend/state, or author an equivalent
> root that composes the same modules. The commands below assume you are in such a root.

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
cp docs/operations/gcp/examples/greenfield.tfvars.example <your-gcp-full-root>/terraform.tfvars
```

A complete, copy-pasteable greenfield config is in
[`examples/greenfield.tfvars.example`](./examples/greenfield.tfvars.example).

---

## 3. Single apply

`gcp-full` provisions everything and deploys the satellite in **one `terraform apply`**. The
in-cluster providers (`kubernetes`/`helm`/`kubectl`) take the cluster endpoint/CA from the
cluster-resolver and a fresh `google_client_config` access token; on a fresh state those are
computed values, so Terraform defers the in-cluster resources until after the cluster exists —
all within the same apply. The preflight binary's kubeconfig is rendered during the apply
(`local_file.kubeconfig`, gke-gcloud-auth-plugin auth), and the install tier is fixed to `A`
(a freshly provisioned cluster's deploy identity can always create the cluster-scoped CRD +
ClusterRole), so the platform/workload counts are known at plan time.

```bash
# from your gcp-full composition root
terraform init

# The refs the preflight binary's --cloud=gcp provider reads from env.
export PREFLIGHT_GCP_PROJECT_ID="$(grep -E '^project_id' terraform.tfvars | cut -d'"' -f2)"
export PREFLIGHT_GCP_REGION="$(grep -E '^region' terraform.tfvars | cut -d'"' -f2)"
export PREFLIGHT_GCP_ROUTER_NAME="$(grep -E '^name' terraform.tfvars | cut -d'"' -f2)-router"
# KMS/network refs resolve from state on a re-plan; on the first apply they are
# discovered as the modules create them.

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
# 1. Destroy everything EXCEPT the flow-log bucket and the CryptoKey.
terraform destroy -auto-approve \
  -target=module.workload -target=module.k8s_observability \
  -target=module.k8s_security -target=module.k8s_platform \
  -target=module.preflight -target=module.secrets -target=module.iam \
  -target=module.cluster_resolver -target=module.cluster

# 2. CryptoKey: remove the prevent_destroy lifecycle block and re-apply before destroying it,
#    or leave the key in place (KMS keys are scheduled for destruction, not deleted immediately).
# 3. Flow-log bucket: empty all object versions and destroy module.network once retention elapses;
#    until then the objects cannot be deleted.
```

The `TestGCPFullGreenfield` e2e automates this whole flow against a real project.
