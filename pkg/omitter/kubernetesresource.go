package omitter

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type kubeMetadata struct {
	Namespace string `yaml:"namespace" json:"namespace"`
}

type kubeResource struct {
	ApiVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   kubeMetadata `yaml:"metadata" json:"metadata"`
}

type kubeResourceList struct {
	Items []kubeResource `yaml:"items" json:"items"`
}

type resourceUnmarshaller func(in []byte, out interface{}) (err error)

type kubernetesResourceOmitter struct {
	apiVersion   string
	resourceKind string
	namespaces   map[string]struct{}
}

func (k *kubernetesResourceOmitter) File(_, _ string) (bool, error) {
	return false, nil
}

func (k *kubernetesResourceOmitter) Contents(path string) (bool, error) {
	input, err := ioutil.ReadFile(path)
	if err != nil {
		return false, err
	}

	var unmarshaller resourceUnmarshaller
	switch {
	case strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml"):
		unmarshaller = yaml.Unmarshal
	case strings.HasSuffix(path, ".json"):
		unmarshaller = json.Unmarshal
	default:
		return false, nil
	}
	var resource kubeResource
	var resourceList kubeResourceList
	err = unmarshaller(input, &resource)
	if err != nil {
		// this means that the input is not a kubernetes resource
		return false, nil
	}

	// check if the input was a list type
	if strings.HasSuffix(resource.Kind, "List") && resource.ApiVersion == "v1" {
		err = unmarshaller(input, &resourceList)
		if err != nil {
			return false, err
		}
	} else {
		resourceList = kubeResourceList{Items: []kubeResource{resource}}
	}

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

func NewKubernetesResourceOmitter(apiVersion, resourceKind *string, namespaces []string) (Omitter, error) {
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
