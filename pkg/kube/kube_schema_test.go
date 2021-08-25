package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesResourceReader(t *testing.T) {
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
			file, err := ioutil.TempFile("", "kube-schema-read-*.yaml")
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(file.Name())
			_, err = file.Write([]byte(tc.resource))
			require.NoError(t, err)
			err = file.Close()
			require.NoError(t, err)

			resource, err := ReadKubernetesResourceFromPath(file.Name())
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedOutput, resource)
		})
	}
}

func TestNonYamlJsonReading(t *testing.T) {
	_, err := ReadKubernetesResourceFromPath("some.path")
	require.Equal(t, NoKubernetesResourceError, err)
}
