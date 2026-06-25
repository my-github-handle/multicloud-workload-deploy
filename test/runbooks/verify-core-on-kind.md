# Verify the cloud-agnostic core on kind

This runbook proves the Tier A path end-to-end on a local [kind](https://kind.sigs.k8s.io)
cluster: install the operator chart, apply a `Workload` CR, and confirm the operator reconciles
it into the full child object set with a `Ready` status.

## Prerequisites

- `docker`, `kind`, `kubectl`, `helm`, and `mage` on PATH.
- The operator image built locally: `mage dockerBuild` (tags `ghcr.io/ops-dev/workload-operator:dev`).

## Steps

1. Create a cluster with metrics-server (the HPA needs it):

   ```bash
   kind create cluster --name core
   kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
   kubectl patch -n kube-system deployment metrics-server --type=json \
     -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
   ```

2. Build and load the operator image into the cluster:

   ```bash
   mage dockerBuild
   kind load docker-image ghcr.io/ops-dev/workload-operator:dev --name core
   ```

   > If `kind load` rejects a multi-arch manifest, rebuild a single-arch image:
   > `docker build --load -f operator/Dockerfile -t ghcr.io/ops-dev/workload-operator:dev .`

3. Install the operator (namespace, CRD, namespace-scoped RBAC, controller):

   ```bash
   kubectl create namespace workload-system
   helm install op charts/workload-operator \
     --namespace workload-system \
     --set image.tag=dev --set watchNamespace=default
   ```

4. Apply a `Workload` CR:

   ```bash
   cat <<'EOF' | kubectl apply -f -
   apiVersion: workload.ops.dev/v1
   kind: Workload
   metadata: { name: demo, namespace: default }
   spec:
     image: nginx:1.27
     port: 80
     resources:
       requests:
         cpu: 100m
         memory: 128Mi
       limits:
         cpu: 500m
         memory: 512Mi
     autoscale: { minReplicas: 2, maxReplicas: 5, targetCPUUtilization: 70 }
   EOF
   ```

5. Verify the Tier A result:

   ```bash
   kubectl get deploy,svc,hpa,pdb,networkpolicy -l app.kubernetes.io/instance=demo -n default
   kubectl get workload demo -n default -o jsonpath='{.status.conditions}'
   kubectl get workload demo -n default -o jsonpath='{.status.readyReplicas}'
   ```

   Expect a `Ready=True` condition, all child objects present (including the two
   NetworkPolicies `demo-default-deny` + `demo-allow`), and `status.readyReplicas` converging
   to 2 as the pods come up.

6. Sanity-check render parity (Terraform renders this same chart):

   ```bash
   helm template demo charts/workload \
     --set name=demo --set namespace=default --set image=nginx:1.27 \
     | kubectl diff -f - || true
   ```

   The rendered objects should match what the operator created, modulo owner references and
   server-populated fields.

7. Run the real-world e2e suite against the cluster (operator must be installed, steps 1–3):

   ```bash
   mage testE2E
   # or directly:
   KUBECONFIG=$(kind get kubeconfig --name core) \
     go test -tags e2e ./test/e2e/... -v -timeout 15m
   ```

   The suite applies a `Workload`, waits for the operator to reconcile it, and asserts the full
   child set, `Ready=True`, converged replicas, and the HPA wiring. Override the target with
   `E2E_NAMESPACE`, `E2E_IMAGE`, `E2E_PORT`, or `E2E_RUN_AS_ROOT=true` for images that need root.

8. Cleanup:

   ```bash
   kind delete cluster --name core
   ```
