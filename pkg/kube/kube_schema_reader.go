package kube

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var NoKubernetesResourceError = errors.New("not a k8s resource")

// ReadKubernetesResourceFromPath tries to read a kubernetes resource from the file.
// it will return a NoKubernetesResourceError in case it's not a yml/yaml or json file or when it is not able to parse it into a known schema.
// Otherwise, it will always return a list resource which either contains the list of the advertised Kind and ApiVersion (which is set then),
// or alternatively just a single Item with Kind and ApiVersion being empty.
func ReadKubernetesResourceFromPath(path string) (*ResourceListWithPath, error) {
	var unmarshaller ResourceUnmarshaller
	switch {
	case strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml"):
		unmarshaller = yaml.Unmarshal
	case strings.HasSuffix(path, ".json"):
		unmarshaller = json.Unmarshal
	default:
		return nil, NoKubernetesResourceError
	}

	input, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var resource Resource
	err = unmarshaller(input, &resource)
	if err != nil {
		return nil, NoKubernetesResourceError
	}

	if resource.Kind == "" || resource.ApiVersion == "" {
		return nil, NoKubernetesResourceError
	}

	var resourceList ResourceList
	// check if the input was a list type
	if strings.HasSuffix(resource.Kind, "List") && resource.ApiVersion == "v1" {
		err = unmarshaller(input, &resourceList)
		if err != nil {
			return nil, err
		}
	} else {
		resourceList = ResourceList{Items: []Resource{resource}}
	}

	return &ResourceListWithPath{
		ResourceList: resourceList,
		Path:         path,
	}, nil
}
