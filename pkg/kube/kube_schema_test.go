package kube

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/require"
)

func TestKubernetesResourceReaderYaml(t *testing.T) {
	for _, tc := range []struct {
		name           string
		resource       string
		expectedError  error
		expectedOutput *ResourceList
	}{
		{
			name: "basic secret",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			expectedError: nil,
			expectedOutput: &ResourceList{
				Items: []Resource{
					{
						ApiVersion: "v1",
						Kind:       "Secret",
						Metadata: Metadata{
							Namespace: "kube-system",
						},
					},
				},
			},
		},
		{

			name: "non resource kind",
			resource: `key1:value1
key2:value2
`,
			expectedError:  NoKubernetesResourceError,
			expectedOutput: nil,
		},
		{
			name: "resource list",
			resource: `---
apiVersion: v1
kind: SecretList
items:
    - apiVersion: v1
      kind: Secret
      metadata:
          namespace: kube-system
          name: first
    - apiVersion: v1
      kind: Secret
      metadata:
          namespace: kube-system
          name: second
`,
			expectedError: nil,
			expectedOutput: &ResourceList{
				Items: []Resource{
					{
						ApiVersion: "v1",
						Kind:       "Secret",
						Metadata: Metadata{
							Namespace: "kube-system",
						},
					},
					{
						ApiVersion: "v1",
						Kind:       "Secret",
						Metadata: Metadata{
							Namespace: "kube-system",
						},
					},
				},
			},
		},
		{
			name: "empty resource list",
			resource: `---
apiVersion: v1
kind: SecretList
items: []
`,
			expectedError: nil,
			expectedOutput: &ResourceList{
				Items: []Resource{},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := asYaml(t, tc.resource)
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(file.Name())

			assertOutput(t, file.Name(), tc.expectedError, tc.expectedOutput)

			file, err = fromYamlToJson(t, tc.resource)
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(file.Name())

			assertOutput(t, file.Name(), tc.expectedError, tc.expectedOutput)
		})
	}
}
func TestNonYamlNonJsonReading(t *testing.T) {
	_, err := ReadKubernetesResourceFromPath("some.path")
	require.Equal(t, NoKubernetesResourceError, err)
}

func TestReadNonK8sJSON(t *testing.T) {
	file, err := os.CreateTemp("", "kube-schema-read-*.yaml")
	require.NoError(t, err)
	defer func(name string) {
		_ = os.Remove(name)
	}(file.Name())

	randomJson := `{"status":"success"}`
	_, err = file.WriteString(randomJson)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	_, err = ReadKubernetesResourceFromPath(file.Name())
	assert.Equal(t, NoKubernetesResourceError, err)
}

func assertOutput(t *testing.T, fileName string, expectedError error, expectedOutput *ResourceList) {
	resource, err := ReadKubernetesResourceFromPath(fileName)
	assert.Equal(t, expectedError, err)
	if expectedOutput != nil {
		assert.Equal(t, fileName, resource.Path)
		assert.Equal(t, expectedOutput, &resource.ResourceList)
	} else {
		assert.Nil(t, resource)
	}
}

func asYaml(t *testing.T, resource string) (*os.File, error) {
	file, err := os.CreateTemp("", "kube-schema-read-*.yaml")
	require.NoError(t, err)
	_, err = file.Write([]byte(resource))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return file, err
}

func fromYamlToJson(t *testing.T, resource string) (*os.File, error) {
	file, err := os.CreateTemp("", "kube-schema-read-*.json")
	require.NoError(t, err)
	bytes, err := yaml.YAMLToJSON([]byte(resource))
	require.NoError(t, err)
	_, err = file.Write(bytes)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return file, err
}
