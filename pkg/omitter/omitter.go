package omitter

import "github.com/openshift/must-gather-clean/pkg/kube"

// FileOmitter is the interface for a type which determines if a file should be included in the output
type FileOmitter interface {
	// OmitPath takes the relative path of the file and its return indicates if the file should be omitted.
	OmitPath(path string) (bool, error)
}

// KubernetesResourceOmitter is the interface for a type which determines whether a k8s resource should be omitted
type KubernetesResourceOmitter interface {
	// OmitKubeResource takes a resource list (which can contain a single resource) and returns whether the resource should be omitted.
	OmitKubeResource(resourceList *kube.ResourceListWithPath) (bool, error)
}

type ReportingOmitter interface {
	FileOmitter
	KubernetesResourceOmitter

	// Report should return all paths that were omitted
	Report() []string
}
