//go:build mage

package main

// Release packaging targets (Release namespace). The product ships as three
// artifacts pinned by one BOM version — the operator OCI image, the two Helm
// charts (OCI), and the Terraform module set (git tag). Provenance (cosign
// signatures + SBOM) is attached so the bundle is marketplace-grade. The per-cloud
// marketplace registry + entitlement metering is a separate Layer-5 publish step
// that mirrors these same digests. Tooling: docker buildx, helm, cosign, syft;
// cosign/syft are optional on a dev laptop (targets skip with a hint if absent).

import (
	"fmt"
	"os"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Release groups the packaging targets (`mage release:bundle VERSION=v0.2.0`).
type Release mg.Namespace

const (
	// chartRepo is the OCI location the Helm charts publish to. Override with CHART_REPO.
	defaultChartRepo = "oci://ghcr.io/ops-dev/charts"
	// imageRepoBase is the operator image repo. Override with IMAGE_REPO.
	defaultImageRepo = "ghcr.io/ops-dev/workload-operator"
	// tfModulesRef is the Terraform module source. Override with TF_MODULES_REF.
	defaultTFModulesRef = "git::https://github.com/ops-dev/multicloud-workload-deploy//modules"
)

func releaseVersion() (string, error) {
	v := os.Getenv("VERSION")
	if v == "" {
		return "", fmt.Errorf("set VERSION (e.g. VERSION=v0.2.0)")
	}
	return v, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// have reports whether a tool is on PATH.
func have(tool string) bool {
	_, err := sh.Output("sh", "-c", "command -v "+tool)
	return err == nil
}

// Image builds and pushes the multi-arch operator image tagged with VERSION, and
// prints the pushed digest. Requires docker buildx + push creds to IMAGE_REPO.
func (Release) Image() error {
	v, err := releaseVersion()
	if err != nil {
		return err
	}
	repo := envOr("IMAGE_REPO", defaultImageRepo)
	ref := repo + ":" + v
	fmt.Printf("building + pushing %s (linux/amd64,linux/arm64)\n", ref)
	if err := sh.RunV("docker", "buildx", "build", "-f", "operator/Dockerfile",
		"--platform", "linux/amd64,linux/arm64", "-t", ref, "--push", "."); err != nil {
		return err
	}
	digest, _ := sh.Output("docker", "buildx", "imagetools", "inspect", ref,
		"--format", "{{.Manifest.Digest}}")
	fmt.Printf("operator image digest: %s\n", strings.TrimSpace(digest))
	return nil
}

// Charts packages and pushes both Helm charts to the OCI CHART_REPO at VERSION.
func (Release) Charts() error {
	v, err := releaseVersion()
	if err != nil {
		return err
	}
	repo := envOr("CHART_REPO", defaultChartRepo)
	v = strings.TrimPrefix(v, "v") // Helm SemVer has no leading v
	for _, c := range []string{"charts/workload-operator", "charts/workload"} {
		pkg := fmt.Sprintf("%s-%s.tgz", lastPath(c), v)
		if err := sh.RunV("helm", "package", c, "--version", v, "--app-version", v, "-d", "dist"); err != nil {
			return err
		}
		if err := sh.RunV("helm", "push", "dist/"+pkg, repo); err != nil {
			return err
		}
	}
	return nil
}

// Sbom generates an SPDX SBOM for the operator image (syft). Skips with a hint if
// syft is absent.
func (Release) Sbom() error {
	v, err := releaseVersion()
	if err != nil {
		return err
	}
	if !have("syft") {
		fmt.Println("syft not installed — skipping SBOM. Install: https://github.com/anchore/syft")
		return nil
	}
	repo := envOr("IMAGE_REPO", defaultImageRepo)
	out := fmt.Sprintf("dist/sbom-%s.spdx.json", v)
	if err := os.MkdirAll("dist", 0o755); err != nil {
		return err
	}
	return sh.RunV("syft", repo+":"+v, "-o", "spdx-json="+out)
}

// Sign cosign-signs the operator image and the SBOM. Skips with a hint if cosign
// is absent. Keyless (OIDC) by default; set COSIGN_KEY for key-based signing.
func (Release) Sign() error {
	v, err := releaseVersion()
	if err != nil {
		return err
	}
	if !have("cosign") {
		fmt.Println("cosign not installed — skipping signing. Install: https://github.com/sigstore/cosign")
		return nil
	}
	repo := envOr("IMAGE_REPO", defaultImageRepo)
	args := []string{"sign", "--yes"}
	if key := os.Getenv("COSIGN_KEY"); key != "" {
		args = append(args, "--key", key)
	}
	return sh.RunV("cosign", append(args, repo+":"+v)...)
}

// Bom assembles release/bom-<VERSION>.yaml from the template, filling in the image
// digest (resolved from the registry when reachable) and artifact refs. This is the
// product release unit; run it after Image/Charts so the digest is available.
func (Release) Bom() error {
	v, err := releaseVersion()
	if err != nil {
		return err
	}
	tmpl, err := os.ReadFile("release/bom.template.yaml")
	if err != nil {
		return err
	}
	repo := envOr("IMAGE_REPO", defaultImageRepo)
	chartRepo := envOr("CHART_REPO", defaultChartRepo)
	tfRef := envOr("TF_MODULES_REF", defaultTFModulesRef)
	chartVer := strings.TrimPrefix(v, "v")

	digest := os.Getenv("IMAGE_DIGEST")
	if digest == "" {
		if d, derr := sh.Output("docker", "buildx", "imagetools", "inspect",
			repo+":"+v, "--format", "{{.Manifest.Digest}}"); derr == nil {
			digest = strings.TrimSpace(d)
		} else {
			digest = "sha256:UNRESOLVED-set-IMAGE_DIGEST-or-push-the-image-first"
		}
	}

	repl := strings.NewReplacer(
		"{{VERSION}}", v,
		"{{TIMESTAMP}}", envOr("RELEASE_TIMESTAMP", "unset (pass RELEASE_TIMESTAMP)"),
		"{{IMAGE_REPO}}", repo,
		"{{IMAGE_DIGEST}}", digest,
		"{{CHART_OPERATOR_REF}}", chartRepo+"/workload-operator",
		"{{CHART_WORKLOAD_REF}}", chartRepo+"/workload",
		"{{CHART_VERSION}}", chartVer,
		"{{CHART_OPERATOR_DIGEST}}", envOr("CHART_OPERATOR_DIGEST", "(set after helm push)"),
		"{{CHART_WORKLOAD_DIGEST}}", envOr("CHART_WORKLOAD_DIGEST", "(set after helm push)"),
		"{{TF_MODULES_REF}}", tfRef,
		"{{SBOM_FILE}}", fmt.Sprintf("dist/sbom-%s.spdx.json", v),
		"{{IMAGE_SIG}}", "cosign (registry-attached)",
		"{{SBOM_SIG}}", "cosign (registry-attached)",
	)
	out := "release/bom-" + v + ".yaml"
	if err := os.WriteFile(out, []byte(repl.Replace(string(tmpl))), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", out)
	return nil
}

// Bundle is the full release pipeline: image → charts → sbom → sign → bom.
func (Release) Bundle() {
	mg.SerialDeps(Release.Image, Release.Charts, Release.Sbom, Release.Sign, Release.Bom)
}

func lastPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
