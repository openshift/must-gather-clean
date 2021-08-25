package omitter

import "github.com/openshift/must-gather-clean/pkg/kube"

// FileOmitter is the interface for a type which determines if a file should be included in the output
type FileOmitter interface {
	// Omit takes the filename and the path of the file and its return indicates if the file should be included.
	Omit(filename, path string) (bool, error)
}

// KubernetesResourceOmitter is the interface for a type which determines whether a k8s resource should be omitted
type KubernetesResourceOmitter interface {
	Omit(resourceList *kube.ResourceList) (bool, error)
}
