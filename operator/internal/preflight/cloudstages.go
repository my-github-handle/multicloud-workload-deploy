package preflight

import "context"

// PreflightProvider abstracts the cloud-specific preflight checks (stages 0..3). It is defined
// here, in the consumer package, so cloud-provider packages depend on preflight and not the
// reverse. Each method returns CheckResults rather than an error so the report stays the single
// source of truth. Implementations MUST emit the stable result IDs the downstream Terraform and
// per-cloud plans key on (e.g. CheckEgress emits egress.metadata_block / egress.ghcr /
// egress.cloud_api / egress.observability / egress.controlplane_fqdn / egress.firewall_inpath).
type PreflightProvider interface {
	// CheckIdentityPermissions implements Stage 0: the deploy identity holds exactly the
	// least-privilege permissions the path needs — flagging MISSING (red, blocking) and, where
	// detectable, EXCESS (amber; excess detection is partial/deferred).
	CheckIdentityPermissions(ctx context.Context) []CheckResult
	// CheckKMSKey implements Stage 1: the resolved key exists, is enabled, and the deploy
	// identity can Encrypt/Decrypt/GenerateDataKey on it.
	CheckKMSKey(ctx context.Context) []CheckResult
	// CheckSecretsBackend implements Stage 2: the secrets backend is reachable, the CSI/sync
	// mechanism is available, and material is CMK-encrypted.
	CheckSecretsBackend(ctx context.Context) []CheckResult
	// CheckEgress implements Stage 3: egress path present, metadata endpoint blockable, and
	// reachability to required endpoints including the control-plane FQDN. Emits one CheckResult
	// per stable egress.* ID.
	CheckEgress(ctx context.Context) []CheckResult
}

// providerCheck adapts one PreflightProvider method into a Check bound to a stage.
type providerCheck struct {
	stage StageID
	name  string
	fn    func(ctx context.Context) []CheckResult
}

func (c providerCheck) Stage() StageID                        { return c.stage }
func (c providerCheck) Name() string                          { return c.name }
func (c providerCheck) Run(ctx context.Context) []CheckResult { return c.fn(ctx) }

// CloudChecks returns the four cloud-facing checks (stages 0..3) backed by the given provider.
// Stages 4 and 5 are Kubernetes checks (see K8sChecks).
func CloudChecks(p PreflightProvider) []Check {
	return []Check{
		providerCheck{stage: StageIdentity, name: "identity", fn: p.CheckIdentityPermissions},
		providerCheck{stage: StageKMS, name: "kms", fn: p.CheckKMSKey},
		providerCheck{stage: StageSecrets, name: "secrets", fn: p.CheckSecretsBackend},
		providerCheck{stage: StageNetwork, name: "network", fn: p.CheckEgress},
	}
}
