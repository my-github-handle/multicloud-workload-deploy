// Package render turns a WorkloadSpec into Kubernetes objects by rendering the
// embedded charts/workload Helm chart — the single source shared with Terraform.
package render

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadv1 "github.com/ops-dev/multicloud-workload-deploy/operator/api/v1"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/chartfs"
)

// releaseName is a deterministic, tier-independent release name. Templates never reference
// .Release.* — identity flows through .Values.name/.Values.namespace only — so this value only
// satisfies the Helm engine and cannot cause divergence between the operator's in-process render
// and Terraform's helm_release. Terraform must use this same release name for identical output.
const releaseName = "workload"

// Chart renders the workload chart for the given name/namespace/spec and returns
// the typed Kubernetes objects.
func Chart(name, namespace string, spec workloadv1.WorkloadSpec) ([]client.Object, error) {
	ch, err := loadEmbeddedChart()
	if err != nil {
		return nil, err
	}

	values, err := valuesFromSpec(name, namespace, spec)
	if err != nil {
		return nil, err
	}
	if err := chartutil.ValidateAgainstSchema(ch, values); err != nil {
		return nil, fmt.Errorf("values fail chart schema: %w", err)
	}

	// Pass a fixed release name + the target namespace. name/namespace are also carried in
	// .Values (above); templates read only .Values, so ReleaseOptions cannot perturb output.
	rv, err := chartutil.ToRenderValues(ch, values, chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("build render values: %w", err)
	}

	rendered, err := engine.Render(ch, rv)
	if err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	return decode(rendered)
}

func loadEmbeddedChart() (*chart.Chart, error) {
	sub, err := fs.Sub(chartfs.FS, "charts/workload")
	if err != nil {
		return nil, err
	}
	var files []*loader.BufferedFile
	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := fs.ReadFile(sub, path)
		if rerr != nil {
			return rerr
		}
		files = append(files, &loader.BufferedFile{Name: path, Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loader.LoadFiles(files)
}

func valuesFromSpec(name, namespace string, spec workloadv1.WorkloadSpec) (map[string]interface{}, error) {
	v := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"image":     spec.Image,
		"port":      int(spec.Port),
		"autoscale": map[string]interface{}{
			"minReplicas":          int(spec.Autoscale.MinReplicas),
			"maxReplicas":          int(spec.Autoscale.MaxReplicas),
			"targetCPUUtilization": int(spec.Autoscale.TargetCPUUtilization),
		},
		"pdb": map[string]interface{}{"minAvailable": 1},
	}
	if spec.LivenessProbe != nil {
		v["livenessProbe"] = map[string]interface{}{"path": spec.LivenessProbe.Path, "port": int(spec.LivenessProbe.Port)}
	}
	if spec.ReadinessProbe != nil {
		v["readinessProbe"] = map[string]interface{}{"path": spec.ReadinessProbe.Path, "port": int(spec.ReadinessProbe.Port)}
	}
	// Ingress: only enabled when the spec requests it; the chart renders no Ingress otherwise.
	if spec.IngressClass != "" {
		v["ingressClass"] = spec.IngressClass
	}
	if spec.Ingress != nil {
		path := spec.Ingress.Path
		if path == "" {
			path = "/"
		}
		pathType := spec.Ingress.PathType
		if pathType == "" {
			pathType = "Prefix"
		}
		v["ingress"] = map[string]interface{}{
			"enabled":  true,
			"host":     spec.Ingress.Host,
			"path":     path,
			"pathType": pathType,
		}
	}
	// Plumb spec.Resources through to .Values.resources so the chart's `with .Values.resources`
	// renders requests/limits. Only set it when non-empty so the chart's `with` guard skips an
	// absent block. A conversion failure is returned, not swallowed: silently dropping resources
	// would render an under-provisioned manifest while reconcile appears to succeed.
	if spec.Resources.Requests != nil || spec.Resources.Limits != nil {
		res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec.Resources)
		if err != nil {
			return nil, fmt.Errorf("convert spec.resources: %w", err)
		}
		if len(res) > 0 {
			v["resources"] = res
		}
	}
	// Security-context overrides: only set them when present so the chart's hardened default
	// applies otherwise. A conversion failure is returned so an override is never silently lost.
	if spec.SecurityContext != nil {
		sc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec.SecurityContext)
		if err != nil {
			return nil, fmt.Errorf("convert spec.securityContext: %w", err)
		}
		v["securityContext"] = sc
	}
	if spec.PodSecurityContext != nil {
		psc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec.PodSecurityContext)
		if err != nil {
			return nil, fmt.Errorf("convert spec.podSecurityContext: %w", err)
		}
		v["podSecurityContext"] = psc
	}
	return v, nil
}

func decode(rendered map[string]string) ([]client.Object, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = workloadv1.AddToScheme(scheme)

	var objs []client.Object
	for name, content := range rendered {
		if len(bytes.TrimSpace([]byte(content))) == 0 {
			continue
		}
		// A single rendered template file may contain multiple YAML documents
		// separated by `---` (e.g. the NetworkPolicy file emits a default-deny plus
		// an allow policy). Loop the decoder until io.EOF so every document is
		// captured — decoding only once would silently drop all but the first object.
		// The chart is the single source rendered by both the operator here and
		// Terraform, so a dropped document is a correctness bug across both paths.
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(content), 4096)
		for {
			raw := map[string]interface{}{}
			err := decoder.Decode(&raw)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", name, err)
			}
			if len(raw) == 0 {
				// Empty document (e.g. a `---` separator with only comments, or a
				// template guarded entirely off). Skip it, keep reading.
				continue
			}
			typed, err := typedFromRaw(scheme, raw)
			if err != nil {
				return nil, fmt.Errorf("type %s: %w", name, err)
			}
			objs = append(objs, typed)
		}
	}
	return objs, nil
}

func typedFromRaw(scheme *runtime.Scheme, raw map[string]interface{}) (client.Object, error) {
	u := &unstructured.Unstructured{Object: raw}
	gvk := u.GroupVersionKind()
	typed, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, typed); err != nil {
		return nil, err
	}
	co, ok := typed.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object %s is not a client.Object", gvk)
	}
	// ensure GVK is set for downstream consumers (scheme.New clears it)
	co.GetObjectKind().SetGroupVersionKind(gvk)
	return co, nil
}
