package omitter

import (
	"sync"

	"github.com/openshift/must-gather-clean/pkg/kube"
)

type MultiReportingOmitter struct {
	fileOmitters []FileOmitter
	k8sOmitters  []KubernetesResourceOmitter

	omittedPathsLock sync.Mutex
	omittedPaths     []string
}

func (m *MultiReportingOmitter) OmitPath(path string) (bool, error) {
	for _, o := range m.fileOmitters {
		omit, err := o.OmitPath(path)
		if err != nil {
			return false, err
		}

		if omit {
			m.appendUnderLock(path)
			return true, nil
		}
	}
	return false, nil
}

func (m *MultiReportingOmitter) OmitKubeResource(resourceList *kube.ResourceListWithPath) (bool, error) {
	for _, o := range m.k8sOmitters {
		omit, err := o.OmitKubeResource(resourceList)
		if err != nil {
			return false, err
		}

		if omit {
			m.appendUnderLock(resourceList.Path)
			return true, nil
		}
	}
	return false, nil
}

func (m *MultiReportingOmitter) Report() []string {
	m.omittedPathsLock.Lock()
	defer m.omittedPathsLock.Unlock()

	var copySlice []string
	for i := 0; i < len(m.omittedPaths); i++ {
		copySlice = append(copySlice, m.omittedPaths[i])
	}

	return copySlice
}

func (m *MultiReportingOmitter) appendUnderLock(path string) {
	m.omittedPathsLock.Lock()
	defer m.omittedPathsLock.Unlock()

	m.omittedPaths = append(m.omittedPaths, path)
}

func NewMultiReportingOmitter(fileOmitters []FileOmitter, k8sOmitters []KubernetesResourceOmitter) ReportingOmitter {
	return &MultiReportingOmitter{
		fileOmitters:     fileOmitters,
		k8sOmitters:      k8sOmitters,
		omittedPathsLock: sync.Mutex{},
		omittedPaths:     []string{},
	}
}
