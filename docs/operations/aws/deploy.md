# Deploy on AWS (`aws-full` greenfield)

End-to-end: provision a complete, secure satellite on AWS from nothing — a hardened private VPC, a
private EKS cluster, encryption, identity, and the cloud-agnostic Layer-3 deploy on top — then
verify and operate it.

> **Already have a cluster?** Use the BYOC fast path instead — a single `terraform apply` against
> the existing cluster. See [`../common/verify-on-kind.md`](../common/verify-on-kind.md) (it
> generalizes to any existing EKS/GKE/AKS). This page is the **greenfield** path that provisions
> the infrastructure too.

Architecture: [`../../architecture/aws.md`](../../architecture/aws.md) · Preflight gate:
[`../common/preflight.md`](../common/preflight.md) · Operator/workload ops:
[`../common/workload-operator.md`](../common/workload-operator.md).

---

## 1. What it provisions

```
phase1-infra:  network → kms → cluster (VPC CNI custom networking) → iam → kubeconfig
phase2-deploy: secrets → preflight (full mode, AWS provider) →
               Layer-3 (operator, security, observability, workload)
```

- **network** — VPC with a primary/secondary CIDR split, per-AZ NAT + AWS Network Firewall
  (FQDN allowlist, default-deny egress), and VPC Flow Logs to a retention-locked S3 bucket.
- **kms** — a customer-managed key (rotation on) used for envelope encryption.
- **iam** — IRSA role + wildcard-free runtime/deploy policies as reviewable artifacts.
- **secrets** — Secrets Manager secrets (CMK-encrypted) + the Secrets Store CSI class.
- **cluster** — hardened private EKS (OIDC/IRSA, CMK secrets encryption, control-plane logging) with
  the **VPC CNI in custom-networking mode** (pods on the secondary CIDR), plus `coredns`/`kube-proxy`.

The VPC CNI is installed with the cluster, so nodes are `Ready` from the start and pods land on the
secondary CIDR — no bootstrap gap. **Cilium** is optional (default off): a chaining-mode overlay on
top of the VPC CNI for NetworkPolicy / Hubble / `toFQDNs`; it does not own the datapath.

---

> The shipped greenfield entry point is [`roots/aws-full`](../../../roots/aws-full). It uses local
> state by default for reviewability; production installs should copy the roots or add a backend
> block that matches the customer's state policy.

## 2. Prerequisites

- `terraform >= 1.7`, the `aws` CLI, `kubectl`, `helm`, `go` (to build the preflight binary).
- AWS credentials for an identity holding the deploy-time permissions. The `iam` module renders that
  permission set to `modules/aws/iam/artifacts/deploy-policy.json` **during `terraform apply`** (the
  file is generated, not checked in). Provisioning EKS + Network Firewall + NAT incurs real cost.
- The operator image is distributed **privately**; supply pull credentials so the root creates a
  docker-registry secret and wires it onto the operator ServiceAccount.

```bash
mage preflightBuild   # builds operator/bin/preflight
# Seed terraform.tfvars from the example and edit the placeholders:
cp roots/aws-full/phase1-infra/terraform.tfvars.example roots/aws-full/phase1-infra/terraform.tfvars
cp roots/aws-full/phase2-deploy/terraform.tfvars.example roots/aws-full/phase2-deploy/terraform.tfvars
```

The operations example [`examples/greenfield.tfvars.example`](./examples/greenfield.tfvars.example)
contains the same workload values in a single file for comparison.

---

## 3. Two-phase apply

`aws-full` is intentionally split into two Terraform roots. Phase 1 provisions cloud resources and
writes the kubeconfig. Phase 2 reads phase-1 state, runs preflight against the live EKS cluster,
creates secret material, and deploys the cloud-agnostic Layer 3.

```bash
# phase 1: cloud infrastructure
cd roots/aws-full/phase1-infra
terraform init
terraform apply -auto-approve

# phase 2: preflight + Kubernetes deploy
cd ../phase2-deploy
terraform init

# The refs the preflight binary's --cloud=aws provider reads from env (the egress
# stage asserts firewall-in-path + metadata-block).
export AWS_REGION="$(grep -E '^region' terraform.tfvars | cut -d'"' -f2)"
# Which concerns this apply PROVISIONS (vs BYO) — scopes the Stage-0 permission
# probe. Greenfield is all four; for a BYO mix, list only what you provision.
export PREFLIGHT_AWS_PROVISION_CONCERNS="kms,secrets,iam,cluster"
terraform apply -auto-approve
```

Expected: `terraform output preflight_verdict` is `green` or `amber` (full mode downgrades
provisioned cloud stages to informational); `terraform output install_tier` is `A`.

> The VPC CNI (custom networking) is installed with the cluster, so nodes are `Ready` and pods land
> on the secondary CIDR with no bootstrap gap. Cilium, if enabled, chains on top and never gates
> node readiness.

---

## 4. Operating notes (AWS-specific)

- **Private cluster access.** The EKS API endpoint is private by default, so `kubectl` works from
  **inside the VPC** (a bastion/runner, or CodeBuild in the VPC). For a one-off check from outside,
  scope the public endpoint to your IP — do **not** commit the IP — then revert it:
  ```bash
  terraform apply -auto-approve \
    -var endpoint_public_access=true \
    -var "public_access_cidrs=[\"$(curl -s https://checkip.amazonaws.com)/32\"]"
  ```
- **Egress is default-deny.** Only allowlisted FQDNs leave the VPC; add to `egress_allowed_fqdns`
  and re-apply. When Cilium chaining is enabled, its `toFQDNs` policy is the in-cluster second layer.
  Confirm each node/pod-subnet route table has a `0.0.0.0/0` route to the firewall **VPC endpoint**
  (not the NAT), and that the firewall rule group reached `ACTIVE`.
- **Private operator image.** Rotate the pull-secret credentials by updating
  `operator_image_pull_secret` and re-applying.
- **Reviewable least-privilege artifacts.** `terraform apply` renders the runtime + deploy-time
  policies under `modules/aws/iam/artifacts/` (generated, not checked in) — scoped to the resolved
  ARNs, no wildcards — for inspection before granting. The committed source of truth is the `iam`
  module and its `tests/no_wildcards.tftest.hcl` golden test.

Workload lifecycle, the preflight gate, and post-deploy verification are covered by the common
runbooks: [`../common/workload-operator.md`](../common/workload-operator.md) and
[`../common/preflight.md`](../common/preflight.md).

---

## 5. BYO variations (deploy into customer-owned infra)

The pieces — `network_mode` / `kms_mode` / `iam_mode` / `cluster_mode` — are **independent** toggles
(`provision` | `byo`). The preflight cloud stages self-gate: a BYO concern reports **amber (not
applicable)**, never red. Tell the preflight provider what you provision via
`PREFLIGHT_AWS_PROVISION_CONCERNS` (subset of `kms,secrets,iam,cluster`; unset = all = greenfield).
Each scenario below has a complete, copy-pasteable example in [`examples/`](./examples).

| Scenario | What you bring / we provision | Example |
|---|---|---|
| **BYO VPC** | Customer's network; we provision KMS + EKS + secrets + identity | [`byo-vpc.tfvars.example`](./examples/byo-vpc.tfvars.example) |
| **BYO everything** | Customer's VPC, KMS, role, cluster; we only deploy Layer 3 | [`byo-everything.tfvars.example`](./examples/byo-everything.tfvars.example) |

**BYO VPC** — `network-resolver` looks up the VPC + subnets by tag
(`kubernetes.io/role/internal-elb` for nodes, `kubernetes.io/role/cni` for pods). There is no
retention-locked flow-log bucket (customer owns logging); `egress.firewall_inpath` is amber:

```hcl
network_mode = "byo"
byo_vpc_id   = "vpc-0abc123..."
```

**BYO KMS** — resolve a customer key by ARN (or alias):

```hcl
kms_mode         = "byo"
byo_kms_key_arn  = "arn:aws:kms:us-east-1:111122223333:key/abcd-..."
```

**BYO cluster / everything** — set the remaining modes to `byo` and supply the `byo_*` values
(`byo_vpc_id` / `byo_kms_key_arn` / `byo_role_arn` / `byo_cluster_name`). Every cloud stage reports
amber; the deploy reduces to the Kubernetes stages. Set `PREFLIGHT_AWS_PROVISION_CONCERNS=""`
(explicit empty). Prefer the BYOC fast path (a single apply, no cloud provider) unless you
specifically need the `aws-full` composition.

---

## 6. Teardown (the flow-log bucket does NOT auto-destroy)

A plain `terraform destroy` will FAIL on the flow-log bucket: it has `force_destroy = false` AND
COMPLIANCE-mode Object Lock, so its objects cannot be deleted — by anyone, including the account
root — until their retention period elapses. This is the point of the retention-locked audit floor;
it is intentional.

```bash
# 1. Destroy phase 2 first (Layer 3 + secrets).
cd roots/aws-full/phase2-deploy
terraform destroy -auto-approve

# 2. Destroy phase 1 EXCEPT the flow-log bucket and its dependents.
cd ../phase1-infra
terraform destroy -auto-approve \
  -target=module.iam -target=module.cluster \
  -target=module.kms

# 3. The flow-log bucket:
#  (a) RETENTION ELAPSED — empty all object versions, then destroy module.network.
#  (b) RETENTION NOT ELAPSED — you cannot delete the objects yet. Wait out the window,
#      or remove the bucket from state (terraform state rm 'module.network[0].aws_s3_bucket.flow_logs'
#      + related resources) and delete it manually once retention lapses.
```

> A Secrets Manager secret tears down with a recovery window. To re-create a same-named secret
> before the window elapses: `aws secretsmanager delete-secret --secret-id <name> --force-delete-without-recovery`.

The `TestAWSFullGreenfield` e2e (`E2E_AWS=true mage testE2EAWS`) automates this whole flow against a
real account.
