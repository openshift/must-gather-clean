package schema

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sigs.k8s.io/yaml"
)

func ReadConfigFromPath(path string) (*SchemaJson, error) {

	extension := filepath.Ext(path)
	isYaml := isYamlExtension(extension)
	if extension != ".json" && !isYaml {
		return nil, fmt.Errorf("unsupported extension \"%s\" found in path \"%v\". Only .json, .yaml and .yml are supported", extension, path)
	}

	var bytes []byte
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if isYaml {
		bytes, err = yaml.YAMLToJSON(bytes)
		if err != nil {
			return nil, err
		}
	}

	schema := &SchemaJson{}
	err = schema.UnmarshalJSON(bytes)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func isYamlExtension(extension string) bool {
	return extension == ".yaml" || extension == ".yml"
}
