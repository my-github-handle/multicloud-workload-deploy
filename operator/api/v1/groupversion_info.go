// Package v1 contains the Workload API for group workload.ops.dev.
//
// +kubebuilder:object:generate=true
// +groupName=workload.ops.dev
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group/version for this API.
	GroupVersion = schema.GroupVersion{Group: "workload.ops.dev", Version: "v1"}

	// SchemeBuilder registers the API types.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group/version to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
