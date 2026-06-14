// Command preflight runs the staged preflight checks and prints a flat JSON object of string→string
// on stdout for the Terraform external data source: {"verdict":"...","report_json":"<Report as JSON string>"}.
//
// It ALWAYS prints a valid flat-string-map to stdout and ALWAYS exits 0 when invoked by Terraform
// (the default) so the external data source never errors — even on an internal failure (bad
// kubeconfig, unimplemented --cloud, etc.), which is surfaced as a RED verdict in the emitted
// report rather than a process error. The Terraform gate keys on the parsed verdict, not the exit
// code. --exit-on-red (default false) is for standalone CLI use, where a non-zero exit on red is
// the convenient behavior.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/fake"
	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

type options struct {
	kubeconfig string
	namespace  string
	mode       string // agnostic | full
	cloud      string // "" (fake) | aws | gcp | azure
	exitOnRed  bool
	// skipK8sWhenNoKubeconfig is a TEST-ONLY escape hatch: when true and no kubeconfig is set, the
	// kubernetes stages are omitted entirely (so cloud-only unit tests don't need a cluster). It
	// is NOT settable from the CLI; real invocations with no kubeconfig get a blocking red Stage 4.
	skipK8sWhenNoKubeconfig bool
}

func main() {
	var opts options
	flag.StringVar(&opts.kubeconfig, "kubeconfig", "", "path to kubeconfig; empty disables Kubernetes stages 4-5")
	flag.StringVar(&opts.namespace, "namespace", "default", "target workload namespace")
	flag.StringVar(&opts.mode, "mode", "agnostic", "agnostic | full (full marks provisioned stages informational)")
	flag.StringVar(&opts.cloud, "cloud", "", "aws | gcp | azure; empty uses the fake provider")
	flag.BoolVar(&opts.exitOnRed, "exit-on-red", false, "exit non-zero on a red verdict (standalone CLI use)")
	flag.Parse()

	out, err := run(context.Background(), opts)
	if err != nil {
		// An internal failure (bad kubeconfig, unimplemented --cloud, client build error) must NOT
		// make the Terraform external data source error: that would abort the plan with an opaque
		// provider error instead of the staged report. Instead we emit a RED verdict carrying the
		// error as a check result and exit 0 (unless --exit-on-red). The TF gate keys on
		// verdict=="red" and blocks the apply with the readable message.
		fmt.Fprintf(os.Stderr, "preflight: %v\n", err)
		out = emitError(err)
	}
	fmt.Println(out)

	if opts.exitOnRed {
		var flat map[string]string
		if json.Unmarshal([]byte(out), &flat) == nil && flat["verdict"] == string(preflight.StatusRed) {
			os.Exit(1)
		}
	}
}

// run assembles the checks for the given options, executes them, and returns the flat-string-map
// JSON. It does not exit the process.
func run(ctx context.Context, opts options) (string, error) {
	provider, err := selectProvider(opts.cloud)
	if err != nil {
		return "", err
	}

	checks := preflight.CloudChecks(provider)

	if opts.kubeconfig != "" {
		cfg, kc, ac, kerr := buildKubeClients(opts.kubeconfig)
		if kerr != nil {
			return "", fmt.Errorf("build kube clients: %w", kerr)
		}
		checks = append(checks, preflight.K8sChecks(kc, ac, cfg, opts.namespace)...)
	} else if !opts.skipK8sWhenNoKubeconfig {
		// The kubernetes stages are the PRIMARY deploy target. In real use a missing --kubeconfig
		// means "no cluster to deploy to", which MUST be a red Stage 4 — NOT a silent skip that
		// emits a false 6-stage green "go". Silent-skip is reserved for explicit test contexts
		// that set skipK8sWhenNoKubeconfig=true.
		checks = append(checks, preflight.NoKubeconfigK8sChecks()...)
	}

	runner := preflight.NewRunner(checks)

	var report preflight.Report
	switch opts.mode {
	case "full":
		// Greenfield <cloud>-full: the cloud stages (0..3) are provisioned by this same apply, so
		// they are informational — a red there must not short-circuit and skip the kubernetes
		// deploy target. The Runner marks them non-blocking (true severity preserved in the
		// report, gating masked by ComputeVerdict) and keeps running stages 4..5 (still blocking).
		report = runner.RunWithProvisionedStages(ctx,
			preflight.StageIdentity, preflight.StageKMS,
			preflight.StageSecrets, preflight.StageNetwork)
	case "agnostic": // BYOC: every stage blocks.
		report = runner.Run(ctx)
	default:
		// Reject a typo'd mode rather than silently defaulting to agnostic, which could apply the
		// wrong gating behavior.
		return "", fmt.Errorf("unknown --mode %q (want agnostic|full)", opts.mode)
	}

	return emit(report)
}

// selectProvider returns the cloud.PreflightProvider for the given --cloud value. When unset, the
// fake provider is used so the binary runs without cloud creds. Real cloud providers are added
// per cloud.
func selectProvider(name string) (cloud.PreflightProvider, error) {
	switch name {
	case "":
		return &fake.Provider{}, nil
	case "aws", "gcp", "azure":
		return nil, fmt.Errorf("cloud provider %q not yet implemented; omit --cloud to use the fake provider", name)
	default:
		return nil, fmt.Errorf("unknown --cloud value %q (want aws|gcp|azure or empty)", name)
	}
}

// emit serializes the report into the flat string→string object the external provider requires:
// report_json is the Report double-encoded as a JSON string.
func emit(report preflight.Report) (string, error) {
	inner, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	flat := map[string]string{
		"verdict":     string(report.Verdict),
		"report_json": string(inner),
	}
	out, err := json.Marshal(flat)
	if err != nil {
		return "", fmt.Errorf("marshal flat output: %w", err)
	}
	return string(out), nil
}

// emitError builds a RED report carrying the failure as a single check result and returns the
// flat-string-map JSON. It is used when run() fails so the Terraform external data source still
// receives a well-formed, gate-able result (verdict=red) instead of a process error. It never
// returns an error itself: if marshaling the synthesized report somehow fails, it falls back to a
// static red literal so stdout is ALWAYS valid flat JSON.
func emitError(runErr error) string {
	report := preflight.Report{
		Verdict: preflight.StatusRed,
		Stages: []preflight.Stage{{
			ID:   preflight.StageIdentity,
			Name: "preflight",
			// Blocking so the stage is a genuine gating red under the blocking-mask model — an
			// internal failure must not be recomputed as merely informational by a downstream
			// consumer that re-runs ComputeVerdict.
			Status:   preflight.StatusRed,
			Blocking: true,
			Results: []preflight.CheckResult{{
				ID:          "preflight.internal_error",
				Status:      preflight.StatusRed,
				Message:     fmt.Sprintf("preflight could not run: %v", runErr),
				Remediation: "fix the reported error (e.g. kubeconfig path, credentials, --cloud value) and re-run terraform plan",
			}},
		}},
	}
	out, err := emit(report)
	if err != nil {
		// Last-resort static fallback — guarantees valid flat JSON on stdout.
		return `{"verdict":"red","report_json":"{\"verdict\":\"red\",\"stages\":[]}"}`
	}
	return out
}

// buildKubeClients constructs the typed and apiextensions clients from a kubeconfig path.
func buildKubeClients(kubeconfig string) (*rest.Config, kubernetes.Interface, apiextclient.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, nil, err
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	ac, err := apiextclient.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	return cfg, kc, ac, nil
}
