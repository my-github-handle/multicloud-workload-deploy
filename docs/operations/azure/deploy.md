# Deploy on Azure (`azure-full` greenfield)

End-to-end: provision a complete, secure satellite on Azure from nothing — a private AKS cluster
with a hardened network, encryption, identity, and the cloud-agnostic Layer-3 deploy on top — then
verify and operate it.

> **Already have a cluster?** Use the BYOC fast path instead — a single `terraform apply` against
> the existing cluster. See [`../common/verify-on-kind.md`](../common/verify-on-kind.md) for that
> flow (it generalizes to any existing EKS/GKE/AKS). This page is the **greenfield** path that
> provisions the infrastructure too.

Architecture: [`../../architecture/azure.md`](../../architecture/azure.md) · Preflight gate:
[`../common/preflight.md`](../common/preflight.md) · Operator/workload ops:
[`../common/workload-operator.md`](../common/workload-operator.md).

---

## 1. What it provisions

```
resource group → log analytics → network → kms → cluster →
        (network-resolver, cluster-resolver) → iam → secrets →
        preflight (full mode, Azure provider) →
        Layer-3 (operator, security, observability, workload)
```

- **network** — VNet (nodes only; pods use the CNI overlay), NAT gateway, Azure Firewall
  (FQDN/CIDR allowlist, default-deny via UDR), and VNet flow logs to a time-based-immutable
  (WORM) Storage container.
- **kms** — a customer-managed Key Vault key (rotation on, purge protection) for envelope
  encryption.
- **iam** — UAMI + federated identity credential + a wildcard-free custom runtime role scoped to
  the vault, plus reviewable deploy/runtime JSON artifacts.
- **secrets** — Key Vault secrets (CMK-encrypted) + the Secrets Store CSI class.
- **cluster** — hardened private AKS (OIDC + Workload Identity, CMK disk encryption, managed RBAC,
  control-plane diagnostics) with **Azure CNI Overlay + the Cilium dataplane** (pods on the
  overlay CIDR).
- **Cilium** — the AKS **dataplane**, selected at cluster creation. There is **no** separate Cilium
  Helm release.

---

> The **greenfield composition root** (`azure-full`) is consumer-owned scaffolding, not shipped
> product code — the shipped product is the `modules/azure/*` building blocks + the charts. Copy the
> reference composition into your own IaC repo, wire your backend/state, or author an equivalent
> root that composes the same modules. The commands below assume you are in such a root.

## 2. Prerequisites

- `terraform >= 1.7`, the `az` CLI, `kubectl`, `kubelogin`, `helm`, `go` (to build the preflight
  binary).
- Azure credentials for an identity holding the deploy-time policy (rendered to
  `artifacts/iam/deploy-policy.json` on apply). Provisioning AKS + Azure Firewall + NAT + Key Vault
  incurs real cost.
- The operator image reachable by the cluster (public, or private with an image pull secret).

```bash
mage preflightBuild   # builds operator/bin/preflight
az login && az account set --subscription <SUBSCRIPTION_ID>
# In your azure-full composition root, create terraform.tfvars from
# terraform.tfvars.example: subscription_id, tenant_id, location, name,
# workload_spec_yaml, control_plane_fqdn, secrets.
```

---

## 3. Single apply (greenfield)

`azure-full` is a **single `terraform apply`** for greenfield:

- **kubeconfig** is generated **in-graph** from the cluster output (`local_sensitive_file`) — no
  manual `az aks get-credentials`. The `kubernetes`/`helm`/`kubectl` providers configure from the
  `cluster-resolver` outputs (resolved at apply).
- **install tier** defaults to `install_tier_override = "A"`, making `install_tier` plan-time-known
  so the Layer-3 `count`s resolve in one apply. Preflight still runs as the gate — `fail_on_red`
  blocks a red verdict; the override only fixes the A/B tier.
- **Cilium** is the AKS dataplane, created with the cluster — no Cilium Helm release; nodes are
  `Ready` and pods land on the overlay CIDR from the start.

> **kubelogin required.** The default cluster is Entra-only (`local_account_disabled = true`), so the
> in-graph kubeconfig is the kubelogin exec form. `kubelogin` must be on `PATH` for the preflight
> binary's k8s stages and the CRD-Established wait. The Terraform k8s providers also use the
> resolver's exec auth, so kubelogin is needed regardless.
>
> **Authorization.** With Azure RBAC for Kubernetes enabled, grant the deploying identity
> *Azure Kubernetes Service RBAC Cluster Admin* on the cluster (or set `admin_group_object_ids` on
> the cluster module) so the providers can create namespaced resources.

```bash
# from your azure-full composition root
terraform init

# The refs the binary's --cloud=azure provider reads from env (resolved-resource
# names follow the `name` prefix; export before apply).
NAME=workload   # match var.name
export AZURE_SUBSCRIPTION_ID="$(az account show --query id -o tsv)"
export PREFLIGHT_AZURE_KEY_VAULT_URI="https://${NAME}-kv.vault.azure.net/"
export PREFLIGHT_AZURE_KEY_NAME="workload-cmk"
# Vault/VNet/UAMI refs the binary checks — derivable from the name prefix:
export PREFLIGHT_AZURE_KEY_VAULT_ID="/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${NAME}-rg/providers/Microsoft.KeyVault/vaults/${NAME}-kv"
export PREFLIGHT_AZURE_VNET_ID="/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${NAME}-rg/providers/Microsoft.Network/virtualNetworks/${NAME}-vnet"
export PREFLIGHT_AZURE_UAMI_PRINCIPAL_ID="$(az identity show -g ${NAME}-rg -n ${NAME}-workload --query principalId -o tsv 2>/dev/null)"

# One apply: provision network → kms → cluster → resolvers → iam → secrets,
# generate the kubeconfig in-graph, run the preflight gate (full mode), then the
# Layer-3 operator/security/observability/workload.
terraform apply -auto-approve
```

Expected: `terraform output preflight_verdict` is `green` or `amber` (full mode downgrades
provisioned cloud stages to informational); `terraform output install_tier` is `A`. The Workload
reconciles to `Ready=True`.

> **Private image.** If the operator image is private, pass a pull secret at apply (created in-graph
> and attached to the operator SA) — never commit it:
>
> ```bash
> AUTH=$(printf '%s:%s' "$GHCR_USER" "$GHCR_TOKEN" | base64)
> DOCKERCFG=$(printf '{"auths":{"ghcr.io":{"auth":"%s"}}}' "$AUTH" | base64)
> terraform apply -auto-approve -var "image_pull_secret_b64=$DOCKERCFG"
> ```

> **Deriving the tier.** Set `install_tier_override=""` to derive the tier from the preflight report
> instead of forcing `"A"`; that path needs two applies (`-target=module.preflight`, then a full
> apply).

---

## 4. Operating notes (Azure-specific)

- **Private cluster access.** The API endpoint is private by default, so `kubectl` works from
  **inside the VNet**. For a one-off check from outside, flip the endpoint public scoped to your IP
  — do **not** commit the IP (the root adds the firewall egress IP automatically so nodes keep
  reaching the API server):
  ```bash
  terraform apply -auto-approve \
    -var cluster_private=false \
    -var "cluster_authorized_ip_ranges=[\"$(curl -s https://api.ipify.org)/32\"]" \
    -var "kms_allowed_ip_ranges=[\"$(curl -s https://api.ipify.org)/32\"]"
  ```
- **kubelogin + RBAC.** The Entra-only cluster's kubeconfig uses the kubelogin exec plugin
  (`kubelogin` on `PATH`). Authorize your identity with *Azure Kubernetes Service RBAC Cluster
  Admin* on the cluster, or set `admin_group_object_ids` on the cluster module.
- **Egress is default-deny.** Only allowlisted FQDNs/CIDRs leave the VNet through the Azure
  Firewall; add to `egress_allowed_fqdns` / `egress_allowed_cidrs` and re-apply. The
  `AzureKubernetesService` FQDN tag (AKS control-plane/image egress) is allowed automatically so
  `userDefinedRouting` nodes can bootstrap. Cilium's `toFQDNs` is the in-cluster second layer.
- **Private operator image.** Supply `operator_image_pull_secret` (server/username/password) so the
  root creates a docker-registry secret and wires it onto the operator ServiceAccount; rotate by
  updating the value and re-applying.
- **Reviewable least-privilege artifacts.** `terraform apply` renders the runtime role, deploy-time
  policy, and federated-credential JSON under `artifacts/iam/` (generated, not checked in) —
  enumerated Actions/DataActions, no wildcards, no Owner/Contributor — for inspection before
  granting. The committed source of truth is the `iam` module and its
  `tests/no_wildcards.tftest.hcl` golden test.

Workload lifecycle, the preflight gate, and post-deploy verification are covered by the common
runbooks: [`../common/workload-operator.md`](../common/workload-operator.md) and
[`../common/preflight.md`](../common/preflight.md).

---

## 5. BYO variations (deploy into customer-owned infra)

The four pieces — `network_mode` / `kms_mode` / `iam_mode` / `cluster_mode` — are **independent**
toggles (`provision` | `byo`). Supply the matching `byo_*` values for any concern set to `byo`:

- **BYO VNet** — `network_mode=byo` + `byo_vnet_name`, `byo_vnet_resource_group`,
  `byo_subnet_names`. We provision the rest into the existing network.
- **BYO Key Vault** — `kms_mode=byo` + `byo_key_vault_id`, `byo_key_vault_name`, `byo_key_name`
  (must have purge protection — the preflight precondition enforces it).
- **BYO identity** — `iam_mode=byo` + `byo_uami_id`, `byo_uami_client_id`. The module still emits
  the role + federated-credential JSON artifacts for the customer to attach.
- **BYO cluster** — `cluster_mode=byo` + `byo_cluster_name`. The cluster-resolver looks it up and
  emits the same tagged `{endpoint, ca, auth}` interface; set `cluster_auth_mode` to match the
  cluster's local-account posture.

---

## 6. Teardown

```bash
terraform destroy -auto-approve
# Key Vault has purge protection + soft delete; it stays soft-deleted for the
# retention window after destroy. The flow-log container immutability policy is
# locked=false, so destroy can clean it up (set locked=true out-of-band for a
# production legal-hold-grade WORM lock).
```
