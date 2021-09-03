package omitter

import (
	"github.com/openshift/must-gather-clean/pkg/kube"
)

type NoopOmitter struct {
	Paths []string
}

func (n *NoopOmitter) OmitPath(path string) (bool, error) {
	n.Paths = append(n.Paths, path)
	return false, nil
}

func (n *NoopOmitter) OmitKubeResource(resourceList *kube.ResourceListWithPath) (bool, error) {
	n.Paths = append(n.Paths, resourceList.Path)
	return false, nil
}

func (n *NoopOmitter) Report() []string {
	return n.Paths
}
