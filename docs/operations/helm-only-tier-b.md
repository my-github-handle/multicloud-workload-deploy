# Operating the Helm-Only (Tier B) Path

Deploy a workload with **no operator and no CRD** — `charts/workload` installed directly with
Helm. This is the operator-less floor: it runs under namespace-only permissions where a
cluster-scoped CRD cannot be created.

> Project guide: [`README.md`](./README.md) · Operator (Tier A): [`workload-operator.md`](./workload-operator.md)
> · Design: [`../components/workload-operator.md`](../components/workload-operator.md) §6 Install model.

---

## When to use Tier B

| | Tier A (operator) | Tier B (Helm-only) |
|---|---|---|
| Cluster-scoped CRD/RBAC | required | **not** required |
| Lifecycle | operator reconciles, status, drift correction | driven by `helm`/`terraform apply` |
| `Workload` resource | yes | no — you set chart values directly |
| Autoscaling (HPA) | yes | **yes** |
| Security floor (default-deny NP, metadata block, hardened pods) | yes | yes |

The same `charts/workload` is rendered in both tiers, so the resulting child objects are identical
— only lifecycle ownership differs.

---

## Install

```bash
helm install web charts/workload \
  --namespace <app-namespace> \
  --set name=web --set namespace=<app-namespace> \
  --set image=<registry>/<image>:<tag> --set port=8080 \
  --set autoscale.minReplicas=1 \
  --set autoscale.maxReplicas=5 \
  --set autoscale.targetCPUUtilization=50 \
  --set-json 'resources={"requests":{"cpu":"25m"},"limits":{"cpu":"250m"}}'
```

> **HPA requires CPU requests.** The HorizontalPodAutoscaler computes utilization as a percentage
> of the container's CPU **request**, so `resources.requests.cpu` must be set or the HPA reports
> `<unknown>` and never scales.

For an image that cannot run under the hardened default (needs root or a writable root
filesystem), relax it explicitly:

```bash
  --set-json 'podSecurityContext={"runAsNonRoot":false,"seccompProfile":{"type":"RuntimeDefault"}}' \
  --set-json 'securityContext={"readOnlyRootFilesystem":false,"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}'
```

## Verify

```bash
kubectl -n <app-namespace> get deploy,svc,hpa,pdb,networkpolicy \
  -l app.kubernetes.io/instance=web
```

Expect the Deployment, Service, HPA (`MINPODS/MAXPODS` from your values), PDB, and the two
NetworkPolicies (`web-default-deny`, `web-allow`). The hardened pod security and metadata-IP egress
block apply exactly as in Tier A.

## Autoscaling check (load → scale-up)

```bash
# Generate HTTP load from inside the cluster.
kubectl -n <app-namespace> create deployment web-load --image=busybox:1.36 --replicas=5 -- \
  /bin/sh -c 'while true; do wget -q -O- http://web.<app-namespace>:8080/ >/dev/null 2>&1; done'

# Watch the HPA react (CPU% climbs, REPLICAS rises toward maxReplicas).
kubectl -n <app-namespace> get hpa web -w
```

Validated on a live AKS cluster: under load the HPA drove `web` from **1 → 5** replicas within
~60s. Because there is no operator re-applying the Deployment, nothing competes with the HPA over
`spec.replicas`.

Tear down the load generator when done:

```bash
kubectl -n <app-namespace> delete deployment web-load
```

## Update & uninstall

```bash
# Change image, scale bounds, etc. — Helm re-renders and applies.
helm upgrade web charts/workload -n <app-namespace> --reuse-values --set image=<new>

# Remove the workload and all its child objects.
helm uninstall web -n <app-namespace>
```

## Notes

- The Deployment does **not** pin `spec.replicas`; the HPA owns the replica count. On a Helm
  upgrade, Helm preserves the live replica count rather than resetting it.
- Tier B has no `Workload` status. Observe health via the Deployment/HPA and pod readiness.
- FQDN-granular egress is not enforced in-cluster here (plain NetworkPolicy can't); that is the
  perimeter firewall's job in the cloud building-block layer.
