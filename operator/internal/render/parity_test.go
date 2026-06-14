package render_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/render"
)

// objKey identifies an object independent of operator-added labels/owner-refs.
type objKey struct{ kind, name, namespace string }

func keyOf(o client.Object) objKey {
	return objKey{o.GetObjectKind().GroupVersionKind().Kind, o.GetName(), o.GetNamespace()}
}

// TestTierAvsTierBParity renders the SAME canonical input through render.Chart (Tier A) and
// compares it to the committed `helm template charts/workload` golden file (Tier B). It asserts
// object-level equivalence: same set of {kind,name,namespace} and matching spec, normalizing
// only the labels/owner-references the operator legitimately adds AFTER render. It also asserts
// Tier B emits no cluster-scoped object.
func TestTierAvsTierBParity(t *testing.T) {
	tierA, err := render.Chart("demo", "demo-ns", sampleSpec())
	if err != nil {
		t.Fatalf("Tier A render: %v", err)
	}

	golden, err := os.ReadFile("testdata/golden_tierb.yaml")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	tierB := decodeGolden(t, golden)

	// 1) Same set of {kind,name,namespace}.
	aKeys, bKeys := keySet(tierA), keySetU(tierB)
	if !sameKeySet(aKeys, bKeys) {
		t.Fatalf("object sets differ:\n Tier A: %v\n Tier B: %v", sortedKeys(aKeys), sortedKeys(bKeys))
	}

	// 2) Matching spec AND metadata labels per object. The operator adds owner references only
	//    after render, so at render time both tiers must produce identical spec and identical
	//    labels — comparing spec alone would let a label drift between tiers pass unnoticed.
	bySpec := map[objKey]map[string]interface{}{}
	byLabels := map[objKey]map[string]interface{}{}
	for _, u := range tierB {
		spec, _ := u.Object["spec"].(map[string]interface{})
		bySpec[keyOfU(u)] = spec
		meta, _ := u.Object["metadata"].(map[string]interface{})
		if meta != nil {
			byLabels[keyOfU(u)], _ = meta["labels"].(map[string]interface{})
		}
	}
	for _, o := range tierA {
		ua, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
		aSpec, _ := ua["spec"].(map[string]interface{})
		if want := bySpec[keyOf(o)]; !equalJSON(aSpec, want) {
			t.Errorf("spec mismatch for %v", keyOf(o))
		}
		if want := byLabels[keyOf(o)]; !equalJSON(toIface(o.GetLabels()), want) {
			t.Errorf("label mismatch for %v: Tier A %v vs Tier B %v", keyOf(o), o.GetLabels(), want)
		}
	}

	// 3) Tier B emits NO cluster-scoped object (no namespace OR known cluster-scoped kind).
	clusterScoped := map[string]bool{
		"ClusterRole": true, "ClusterRoleBinding": true,
		"CustomResourceDefinition": true, "Namespace": true,
		"PersistentVolume": true, "Node": true,
	}
	for _, u := range tierB {
		if clusterScoped[u.GetKind()] {
			t.Errorf("Tier B emitted cluster-scoped object %s/%s", u.GetKind(), u.GetName())
		}
		if u.GetNamespace() == "" {
			t.Errorf("Tier B object %s/%s has no namespace (cluster-scoped?)", u.GetKind(), u.GetName())
		}
	}
}

// decodeGolden splits the multi-doc helm output into unstructured objects, skipping empty docs.
func decodeGolden(t *testing.T, data []byte) []*unstructured.Unstructured {
	t.Helper()
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var out []*unstructured.Unstructured
	for {
		raw := map[string]interface{}{}
		err := dec.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode golden: %v", err)
		}
		if len(raw) == 0 {
			continue
		}
		out = append(out, &unstructured.Unstructured{Object: raw})
	}
	return out
}

func keySet(objs []client.Object) map[objKey]bool {
	m := map[objKey]bool{}
	for _, o := range objs {
		m[keyOf(o)] = true
	}
	return m
}

func keySetU(objs []*unstructured.Unstructured) map[objKey]bool {
	m := map[objKey]bool{}
	for _, u := range objs {
		m[keyOfU(u)] = true
	}
	return m
}

func keyOfU(u *unstructured.Unstructured) objKey {
	return objKey{u.GetKind(), u.GetName(), u.GetNamespace()}
}

func sameKeySet(a, b map[objKey]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[objKey]bool) []objKey {
	out := make([]objKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprintf("%v", out[i]) < fmt.Sprintf("%v", out[j])
	})
	return out
}

// equalJSON deep-compares two values via a canonical JSON round-trip, so int/float and
// map-ordering differences between the typed objects (in-process render) and the
// YAML-decoded objects (helm template) do not cause false mismatches. Null values are
// pruned first: marshaling a typed object emits zero-value fields such as
// template.metadata.creationTimestamp as null, whereas YAML decoding omits them — a null
// field is semantically absent, so both forms must compare equal.
func equalJSON(a, b interface{}) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	var an, bn interface{}
	if json.Unmarshal(ab, &an) != nil || json.Unmarshal(bb, &bn) != nil {
		return false
	}
	rean, _ := json.Marshal(pruneNull(an))
	rebn, _ := json.Marshal(pruneNull(bn))
	return bytes.Equal(rean, rebn)
}

// toIface converts a string map to map[string]interface{} for comparison against decoded YAML.
func toIface(m map[string]string) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := map[string]interface{}{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// pruneNull recursively removes map entries whose value is null, so a zero-value field
// rendered as null is treated as absent.
func pruneNull(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, val := range t {
			if val == nil {
				continue
			}
			out[k] = pruneNull(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, val := range t {
			out[i] = pruneNull(val)
		}
		return out
	default:
		return v
	}
}
