# Deploy on AWS (`aws-full` greenfield)

End-to-end: provision a complete, secure satellite on AWS from nothing — a private EKS cluster with
a hardened network, encryption, identity, and the cloud-agnostic Layer-3 deploy on top — then
verify and operate it.

> **Already have a cluster?** Use the BYOC fast path instead — a single `terraform apply` against
> the existing cluster. See [`../common/verify-on-kind.md`](../common/verify-on-kind.md) for that
> flow (it generalizes to any existing EKS/GKE/AKS). This page is the **greenfield** path that
> provisions the infrastructure too.

Architecture: [`../../architecture/aws.md`](../../architecture/aws.md) · Preflight gate:
[`../common/preflight.md`](../common/preflight.md) · Operator/workload ops:
[`../common/workload-operator.md`](../common/workload-operator.md).

---

## 1. What it provisions

```
network → kms → iam → secrets → cluster (incl. vpc-cni custom networking) →
        (network-resolver, cluster-resolver) → preflight (full mode, AWS provider) →
        Layer-3 (operator, security, observability, workload) [+ optional Cilium chaining]
```

- **network** — VPC with a primary/secondary CIDR split, per-AZ NAT + AWS Network Firewall
  (FQDN allowlist, default-deny), and VPC Flow Logs to a retention-locked S3 bucket.
- **kms** — a customer-managed key (rotation on) used for envelope encryption.
- **iam** — IRSA role + wildcard-free deploy/runtime policies as reviewable artifacts.
- **secrets** — Secrets Manager secrets (CMK-encrypted) + the Secrets Store CSI class.
- **cluster** — hardened private EKS (OIDC/IRSA, CMK secrets encryption, control-plane logging) with
  the **VPC CNI in custom-networking mode** (pods on the secondary CIDR), plus `coredns`/`kube-proxy`.
- **Cilium** *(optional, default off)* — chaining mode on top of the VPC CNI for NetworkPolicy /
  Hubble / `toFQDNs`; it does not own the datapath.

---

> The **greenfield composition root** (`aws-full`) is consumer-owned scaffolding, not shipped
> product code — the shipped product is the `modules/aws/*` building blocks + the charts. Copy the
> reference composition into your own IaC repo, wire your backend/state, or author an equivalent
> root that composes the same modules. The commands below assume you are in such a root.

## 2. Prerequisites

- `terraform >= 1.7`, the `aws` CLI, `kubectl`, `helm`, `go` (to build the preflight binary).
- AWS credentials for an identity holding the deploy-time policy (rendered to
  `modules/aws/iam/artifacts/deploy-policy.json` on apply). Provisioning EKS + Network Firewall +
  NAT incurs real cost.
- The operator image reachable by the cluster (public, or private with an image pull secret).

```bash
mage preflightBuild   # builds operator/bin/preflight
# In your aws-full composition root, create terraform.tfvars with:
#   region, name, workload_spec_yaml, control_plane_fqdn, psa_enforce_level, secrets
```

---

## 3. Two-phase apply (NOT single-apply for greenfield)

`aws-full` is a documented **two-phase apply**. The `kubernetes`/`helm`/`kubectl` providers are
configured from `cluster-resolver` outputs (endpoint/CA/auth) that do not exist until the EKS
cluster is created, and Terraform forbids provider config that depends on not-yet-created
resources. (Only the BYOC fast path, against an existing cluster, is genuinely single-apply.)

### Phase 1 — cloud infra incl. the cluster

Leave `create_secret_provider_class = false` (the default): the `secrets` module creates the
Secrets Manager secrets, but not the kubectl `SecretProviderClass`, whose CRD does not exist on the
cluster yet.

```bash
# from your aws-full composition root
terraform init

# Provision: network → kms → iam → secrets(creation) → cluster → resolvers.
# iam and secrets are siblings (no edge), so both apply in Phase 1 without a cycle.
terraform apply -auto-approve \
  -target=module.network -target=module.network_resolver \
  -target=module.kms -target=module.iam -target=module.secrets \
  -target=module.cluster -target=module.cluster_resolver

# Write the kubeconfig the preflight binary + providers use.
aws eks update-kubeconfig --name "$(terraform output -raw workload_name)" \
  --region "$(terraform output -raw region 2>/dev/null || echo us-east-1)" \
  --kubeconfig /tmp/aws-full.kubeconfig
```

> The VPC CNI (custom networking) is installed with the cluster in Phase 1, so nodes are `Ready`
> from the start and pods land on the secondary CIDR — there is no bootstrap gap. Cilium, if
> enabled, chains on top in Phase 2 and never gates node readiness.

### Phase 2 — Layer 3 + the SecretProviderClass

```bash
# Enable the kubectl SecretProviderClass now that the cluster + CSI CRD exist.
echo 'create_secret_provider_class = true' >> terraform.tfvars

# The refs the binary's --cloud=aws provider reads from env (the egress stage
# asserts firewall-in-path + metadata-block).
export AWS_REGION=us-east-1
export PREFLIGHT_AWS_KMS_KEY_ARN="$(terraform output -raw kms_key_arn)"
export PREFLIGHT_AWS_VPC_ID="$(terraform output -raw vpc_id)"
export PREFLIGHT_AWS_EGRESS_PATH_REF="$(terraform output -raw egress_path_ref 2>/dev/null || true)"
export PREFLIGHT_AWS_CONTROL_PLANE_FQDN="satellite.control.ops.dev"
# Which concerns this apply PROVISIONS (vs BYO) — scopes the Stage-0 permission probe.
# Greenfield (all provisioned): all four. For a BYO mix list only what you provision.
export PREFLIGHT_AWS_PROVISION_CONCERNS="kms,secrets,iam,cluster"

# A full apply now reconciles the preflight gate (full mode), the SecretProviderClass,
# all Layer-3 modules, and (if install_cilium=true) the Cilium chaining overlay.
terraform apply -auto-approve
```

Expected: `terraform output preflight_verdict` is `green` or `amber` (full mode downgrades
provisioned cloud stages to informational); `terraform output install_tier` is `A` (the deploy
identity can create the cluster-scoped CRD + ClusterRole on a freshly provisioned cluster).

---

## 4. Verify the satellite + security floor

> **Private cluster.** The EKS API endpoint is private by default, so `kubectl` only works from
> **inside the VPC** (a bastion/runner, or CodeBuild in the VPC). For a one-off check from outside,
> temporarily scope the public endpoint to your IP — do **not** commit the IP:
>
> ```bash
> terraform apply -auto-approve \
>   -var endpoint_public_access=true \
>   -var "public_access_cidrs=[\"$(curl -s https://checkip.amazonaws.com)/32\"]"
> ```

```bash
KCFG=/tmp/aws-full.kubeconfig
kubectl --kubeconfig $KCFG get workload demo -n workload-system \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'      # -> True
kubectl --kubeconfig $KCFG get deploy,svc,hpa,pdb demo -n workload-system
kubectl --kubeconfig $KCFG get networkpolicy -n workload-system        # default-deny + allow
kubectl --kubeconfig $KCFG -n kube-system get pods -l k8s-app=aws-node # VPC CNI running
kubectl --kubeconfig $KCFG get pods -n workload-system -o wide          # pods on 100.64.x (secondary CIDR)
aws s3api get-object-lock-configuration \
  --bucket "$(terraform output -raw flow_log_bucket_arn | cut -d: -f6)"  # COMPLIANCE retention
```

Expect a `Ready=True` Workload; the four child objects; both NetworkPolicies; the VPC CNI running
with workload pods on `100.64.x` IPs; and a COMPLIANCE object-lock rule on the flow-log bucket (the
retention-locked, customer-owned audit floor).

### Egress in-path proof (verify at apply, not just validate)

The firewall is the load-bearing control. From a debug pod:

```bash
# An allowlisted FQDN succeeds; a non-allowlisted one is dropped by the firewall.
kubectl --kubeconfig $KCFG run egress-test --rm -it --image=curlimages/curl --restart=Never -- \
  sh -c 'curl -sS -m 5 https://ghcr.io >/dev/null && echo GHCR_OK; curl -sS -m 5 https://example.com || echo BLOCKED_AS_EXPECTED'
```

Also confirm each node-subnet route table has a `0.0.0.0/0` route to the firewall **VPC endpoint**
(not the NAT), and that the firewall rule group reached `ACTIVE`.

### Reviewable least-privilege artifacts

```bash
cat ../../modules/aws/iam/artifacts/runtime-policy.json   # scoped to resolved ARNs, no wildcards
cat ../../modules/aws/iam/artifacts/deploy-policy.json
cat ../../modules/aws/iam/artifacts/trust-policy.json
```

---

## 5. BYO variations (deploy into customer-owned infra)

The four pieces — `network_mode` / `kms_mode` / `iam_mode` / `cluster_mode` — are **independent**
toggles (`provision` | `byo`). The preflight cloud stages self-gate: a BYO concern reports **amber
(not applicable)**, never red. Tell the preflight provider what you provision via
`PREFLIGHT_AWS_PROVISION_CONCERNS` (subset of `kms,secrets,iam,cluster`; unset = all = greenfield).

**BYO VPC** — customer already has a network; we provision the rest:

```hcl
network_mode = "byo"
byo_vpc_id   = "vpc-0abc123..."
```
`network-resolver` looks up the VPC + subnets by tag (`kubernetes.io/role/internal-elb` for nodes,
`kubernetes.io/role/cni` for pods). `egress.firewall_inpath` is amber (customer owns the edge);
there is no retention-locked flow-log bucket (customer owns logging). Set
`PREFLIGHT_AWS_PROVISION_CONCERNS="kms,secrets,iam,cluster"`.

**BYO cluster** — usually better served by the BYOC fast path (a single apply, no cloud provider).
Use `aws-full` with `cluster_mode = "byo"` only to provision surrounding infra (e.g. a CMK +
secrets) in the same root:

```hcl
cluster_mode     = "byo"
byo_cluster_name = "customer-eks-prod"
```
`cluster-resolver` looks up the cluster's endpoint/CA and emits the same exec auth, so the
Kubernetes providers configure identically. The real gate becomes the Kubernetes stages (4–5):
install tier, namespace, workload-identity binding.

**BYO everything** — fully customer-owned infra:

```hcl
network_mode = "byo"; kms_mode = "byo"; iam_mode = "byo"; cluster_mode = "byo"
# byo_vpc_id / byo_kms_key_arn (or alias) / byo_role_arn / byo_cluster_name
```
Every cloud stage reports amber (not applicable); the deploy reduces to the Kubernetes stages +
the shared-responsibility contract. Set `PREFLIGHT_AWS_PROVISION_CONCERNS=""` (explicit empty).
Prefer the BYOC fast path here unless you specifically need the `aws-full` composition.

---

## 6. Day-2

- **Preflight mode.** `aws-full` runs `--mode=full --cloud=aws`: cloud stages satisfied by
  provisioning are informational (amber), not blocking. A red verdict still blocks unless
  `fail_on_red = false`. See [`../common/preflight.md`](../common/preflight.md).
- **Workload lifecycle.** Tier A on a freshly provisioned cluster — operate the workload exactly as
  in [`../common/workload-operator.md`](../common/workload-operator.md).
- **Egress is default-deny.** Only allowlisted FQDNs leave the VPC; add to `egress_allowed_fqdns`
  and re-apply. When Cilium chaining is enabled, its `toFQDNs` policy is the in-cluster second layer.
- **Audit floor.** VPC Flow Logs are immutable (COMPLIANCE Object Lock) for the retention period —
  this affects teardown (next section).

---

## 7. Teardown (the flow-log bucket does NOT auto-destroy)

A plain `terraform destroy` will FAIL on the flow-log bucket: it has `force_destroy = false` AND
COMPLIANCE-mode Object Lock, so its objects cannot be deleted — by anyone, including the account
root — until their retention period elapses. This is the point of the retention-locked audit floor;
it is intentional.

```bash
# 1. Destroy everything EXCEPT the flow-log bucket and its dependents.
terraform destroy -auto-approve \
  -target=module.workload -target=module.k8s_observability \
  -target=module.k8s_security -target=module.k8s_platform \
  -target=module.preflight -target=module.secrets -target=module.iam \
  -target=module.cluster_resolver -target=module.cluster \
  -target=module.kms

# 2. The flow-log bucket:
#  (a) RETENTION ELAPSED — empty all object versions, then destroy module.network.
#  (b) RETENTION NOT ELAPSED — you cannot delete the objects yet. Wait out the window,
#      or remove the bucket from state (terraform state rm 'module.network[0].aws_s3_bucket.flow_logs'
#      + related resources) and delete it manually once retention lapses.
```

> A Secrets Manager secret tears down with a recovery window. To re-create a same-named secret
> before the window elapses: `aws secretsmanager delete-secret --secret-id <name> --force-delete-without-recovery`.

The `TestAWSFullGreenfield` e2e (`E2E_AWS=true mage testE2EAWS`) automates this whole flow against
a real account.
