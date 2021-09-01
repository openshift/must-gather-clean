package kube

// TODO(tjungblu): check whether we can tap into the OpenShift and Kubernetes api-machinery for this

type Metadata struct {
	Namespace string `yaml:"namespace" json:"namespace"`
}

type Resource struct {
	ApiVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
}

type ResourceList struct {
	Items []Resource `yaml:"items" json:"items"`
}

type ResourceListWithPath struct {
	ResourceList
	Path string
}

// ResourceUnmarshaller is a helper type to abstract yaml and json marshalling
type ResourceUnmarshaller func(in []byte, out interface{}) (err error)
