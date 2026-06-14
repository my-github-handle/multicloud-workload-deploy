# Operations Guide

How to operate the BYOC multi-cloud workload deployment product. This is the whole-project entry
point; component-specific runbooks are linked below.

> Architecture: [`../architecture.md`](../architecture.md) · Design: [`../design.md`](../design.md)
> · Components: [`../components/`](../components/)

---

## Map of operations docs

| Doc | Scope |
|---|---|
| This file | Whole-project: prerequisites, deploy paths, day-2 overview |
| [`workload-operator.md`](./workload-operator.md) | Tier A: operating the operator and `Workload` resources |
| [`helm-only-tier-b.md`](./helm-only-tier-b.md) | Tier B: deploying `charts/workload` with Helm only (no operator/CRD) |
| [`examples/`](./examples) | Runnable, copy-pasteable manifests and commands |
| [`../../test/runbooks/`](../../test/runbooks) | Infra-dependent verification procedures (kind, per-cloud) |

---

## Prerequisites

- `kubectl`, `helm` (v3), and access to a target cluster (`KUBECONFIG`).
- For building/publishing the operator: `go` (1.26), `docker` (with buildx), `mage`.
- The operator image reachable by the cluster — public, or private with an image pull secret.

---

## Deploy paths

The product has two entry shapes (see [`../architecture.md`](../architecture.md) §5). This guide
covers the cloud-agnostic core install that both paths share.

### 1. Install the operator (Tier A)

```bash
# Create the namespace the operator runs in.
kubectl create namespace workload-system

# (private registry only) create a pull secret in the operator namespace.
kubectl -n workload-system create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io --docker-username=<user> --docker-password=<token>

# Install the CRD, controller, and namespace-scoped RBAC.
helm install op charts/workload-operator \
  --namespace workload-system \
  --set image.repository=<registry>/workload-operator \
  --set image.tag=<version> \
  --set watchNamespace=<app-namespace> \
  --set 'imagePullSecrets[0].name=ghcr-pull' \
  --set serviceMonitor.enabled=true        # if the Prometheus Operator CRD is present
```

The operator and the workloads it manages **may live in different namespaces** — set
`watchNamespace` to the app namespace; RBAC is granted there automatically.

### 2. Deploy a workload

```bash
kubectl apply -f docs/operations/examples/workload-basic.yaml
```

See [`examples/`](./examples) for variants (probes, ingress, private/root images, autoscaling).

> **No cluster-scoped permissions?** Use the operator-less **Tier B** path instead — install
> `charts/workload` directly with Helm, no operator or CRD required. See
> [`helm-only-tier-b.md`](./helm-only-tier-b.md). The security floor and HPA work identically.

### 3. Verify

```bash
kubectl -n <app-namespace> get workload,deploy,svc,hpa,pdb,networkpolicy \
  -l app.kubernetes.io/instance=<name>
kubectl -n <app-namespace> get workload <name> \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}'
```

Expect `Ready=True` and the full child set. Every child carries
`app.kubernetes.io/{name,instance,part-of,managed-by}` for fleet-wide querying.

---

## Day-2 operations

| Task | How |
|---|---|
| Update image / scale bounds | edit the `Workload` (`kubectl edit workload <name>`); the operator reconciles |
| Inspect status | `kubectl get workload <name> -o yaml` → `.status.conditions`, `.status.readyReplicas` |
| Diagnose a stuck workload | see [`workload-operator.md`](./workload-operator.md) → Troubleshooting |
| Upgrade the operator | `helm upgrade op charts/workload-operator …` (re-applies the CRD) |
| Uninstall a workload | `kubectl delete workload <name>` — children are garbage-collected via owner refs |
| Uninstall the operator | `helm uninstall op -n workload-system` (CRD removal is manual and deletes all Workloads) |

---

## Security & governance notes

- Workloads run under a hardened pod security context by default; relaxing it is explicit and
  per-workload (`spec.securityContext` / `spec.podSecurityContext`).
- Each workload's pods are default-deny with the cloud metadata endpoint blocked.
- Resources are governed as a unit via the `app.kubernetes.io/instance=<name>` label selector.
