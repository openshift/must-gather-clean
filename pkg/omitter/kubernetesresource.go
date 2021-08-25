package omitter

import (
	"errors"

	"github.com/openshift/must-gather-clean/pkg/kube"
)

type kubernetesResourceOmitter struct {
	apiVersion   string
	resourceKind string
	namespaces   map[string]struct{}
}

func (k *kubernetesResourceOmitter) Omit(resourceList *kube.ResourceList) (bool, error) {
	if len(resourceList.Items) == 0 {
		return false, nil
	}

	var found bool
	// loop over the resources and if one of them matches the criteria then set the `found` flag.
	for _, r := range resourceList.Items {
		// if namespaces are specified then verify that the resource belongs to one of the namespaces
		if len(k.namespaces) > 0 {
			if _, ok := k.namespaces[r.Metadata.Namespace]; !ok {
				continue
			}
		}

		// if not of the specified kind then return
		if k.resourceKind != r.Kind {
			continue
		}

		// if apiVersion is specified and does not match resource apiVersion then return
		if k.apiVersion != "" && k.apiVersion != r.ApiVersion {
			continue
		}

		found = true
		break
	}
	return found, nil
}

func NewKubernetesResourceOmitter(apiVersion, resourceKind *string, namespaces []string) (KubernetesResourceOmitter, error) {
	if resourceKind == nil || *resourceKind == "" {
		return nil, errors.New("no resourceKind specified in omit")
	}
	ns := map[string]struct{}{}
	for _, n := range namespaces {
		ns[n] = struct{}{}
	}
	var version string
	if apiVersion != nil {
		version = *apiVersion
	}
	return &kubernetesResourceOmitter{apiVersion: version, resourceKind: *resourceKind, namespaces: ns}, nil
}
