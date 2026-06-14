package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RolloutStrategy selects how new versions roll out.
// +kubebuilder:validation:Enum=RollingUpdate;Canary
type RolloutStrategy string

const (
	RolloutRollingUpdate RolloutStrategy = "RollingUpdate"
	RolloutCanary        RolloutStrategy = "Canary"
)

// Canary is not silently inert: the operator always renders RollingUpdate in this build, but
// when spec.RolloutStrategy == Canary the reconciler sets a degraded status condition
// (RolloutDegraded=True, reason CanaryUnsupported) so the unsupported request stays visible.

// Probe is a minimal HTTP health-check definition.
type Probe struct {
	// +kubebuilder:validation:Required
	Path string `json:"path"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Port int32 `json:"port"`
}

// Autoscale configures the HorizontalPodAutoscaler bounds.
// +kubebuilder:validation:XValidation:rule="self.maxReplicas >= self.minReplicas",message="maxReplicas must be greater than or equal to minReplicas"
type Autoscale struct {
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilization int32 `json:"targetCPUUtilization"`
}

// WorkloadSpec is the desired state of a Workload.
type WorkloadSpec struct {
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Port int32 `json:"port"`

	// +kubebuilder:default=RollingUpdate
	RolloutStrategy RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// +kubebuilder:validation:Required
	Autoscale Autoscale `json:"autoscale"`

	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext overrides the container security context. When unset, a hardened default
	// applies (non-root, no privilege escalation, read-only root filesystem, all capabilities
	// dropped). Set this only for images that cannot satisfy the hardened default.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// PodSecurityContext overrides the pod security context. When unset, a hardened default
	// applies (run as non-root, RuntimeDefault seccomp).
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// IngressClass names the IngressClass used when Ingress is set (per-cloud ingress controller).
	// Ignored when Ingress is nil.
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`

	// Ingress, when set, renders an Ingress routing the given host/path to the workload Service.
	// When nil, no Ingress is created.
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`
}

// IngressConfig describes how external traffic reaches the workload.
type IngressConfig struct {
	// Host is the DNS name routed to the workload (e.g. app.example.com).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// Path is the HTTP path prefix. Defaults to "/".
	// +kubebuilder:default=/
	// +optional
	Path string `json:"path,omitempty"`

	// PathType is the Kubernetes Ingress path-match type. Defaults to Prefix.
	// +kubebuilder:validation:Enum=Prefix;Exact;ImplementationSpecific
	// +kubebuilder:default=Prefix
	// +optional
	PathType string `json:"pathType,omitempty"`
}

// WorkloadStatus is the observed state of a Workload.
type WorkloadStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`

// Workload is the Schema for the workloads API.
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSpec   `json:"spec,omitempty"`
	Status WorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadList contains a list of Workload.
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workload{}, &WorkloadList{})
}
