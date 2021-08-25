package omitter

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKubernetesResourceOmitter(t *testing.T) {
	for _, tc := range []struct {
		name          string
		apiVersion    string
		kind          string
		namespaces    []string
		expectedError string
	}{
		{
			name:          "all parameters are empty",
			expectedError: "no resourceKind specified in omit",
		},
		{
			name:       "all parameters are specified",
			apiVersion: "machine.openshift.io/v1beta1",
			kind:       "Machine",
			namespaces: []string{"kube-system", "openshift"},
		},
		{
			name:          "only namespaces is specified",
			namespaces:    []string{"kube-system", "openshift"},
			expectedError: "no resourceKind specified in omit",
		},
		{
			name:       "apiVersion and kind is specified but no namespace",
			apiVersion: "machine.openshift.io/v1beta1",
			kind:       "Machine",
		},
		{
			name:          "apiVersion but no kind or namespace",
			apiVersion:    "machine.openshift.io/v1beta1",
			expectedError: "no resourceKind specified in omit",
		},
		{
			name: "kind but no apiVersion or namespace",
			kind: "Machine",
		},
		{
			name:          "namespace, apiVersion specified, no kind",
			apiVersion:    "machine.openshift.io/v1beta1",
			namespaces:    []string{"kube-system", "oepnshift"},
			expectedError: "no resourceKind specified in omit",
		},
		{
			name:       "namespace, kind specified, no apiVersion",
			namespaces: []string{"kube-system", "oepnshift"},
			kind:       "Machine",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewKubernetesResourceOmitter(&tc.apiVersion, &tc.kind, tc.namespaces)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubernetesResourceOmitter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		resource   string
		omit       bool
		apiVersion string
		kind       string
		namespaces []string
	}{
		{
			name: "all match",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			apiVersion: "v1",
			kind:       "Secret",
			namespaces: []string{"kube-system", "openshift"},
			omit:       true,
		},
		{
			name: "namespace match",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			kind:       "Secret",
			namespaces: []string{"kube-system", "openshift"},
			omit:       true,
		},
		{
			name: "namespace mismatch",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: non-secret
`,
			kind:       "Secret",
			namespaces: []string{"kube-system", "openshift"},
			omit:       false,
		},
		{
			name: "kind mismatch",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			kind:       "ConfigMap",
			namespaces: []string{"kube-system", "openshift"},
			omit:       false,
		},
		{
			name: "apiVersion match",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			kind:       "Secret",
			apiVersion: "v1",
			omit:       true,
		},
		{
			name: "apiVersion mismatch",
			resource: `apiVersion: v1
kind: Secret
metadata:
    namespace: kube-system
`,
			kind:       "Secret",
			apiVersion: "v2",
			omit:       false,
		},
		{
			name: "resource list all match",
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
			kind:       "Secret",
			apiVersion: "v1",
			omit:       true,
		},
		{
			name: "resource list one match",
			resource: `---
apiVersion: v1
kind: SecretList
items:
    - apiVersion: v1
      kind: Secret
      metadata:
          namespace: kube-system
          name: first
    - apiVersion: v2
      kind: Secret
      metadata:
          namespace: kube-system
          name: second
`,
			kind:       "Secret",
			apiVersion: "v1",
			omit:       true,
		},
		{
			name: "resource list no match",
			resource: `---
apiVersion: v1
kind: SecretList
items:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
          namespace: kube-system
          name: first
    - apiVersion: v1
      kind: ConfigMap
      metadata:
          namespace: kube-system
          name: second
`,
			kind:       "Secret",
			apiVersion: "v1",
			omit:       false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := ioutil.TempFile("", "resource-omit-*.yaml")
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(file.Name())
			_, err = file.Write([]byte(tc.resource))
			require.NoError(t, err)
			err = file.Close()
			require.NoError(t, err)

			omitter, err := NewKubernetesResourceOmitter(&tc.apiVersion, &tc.kind, tc.namespaces)
			require.NoError(t, err)

			resourceList, err := kube.ReadKubernetesResourceFromPath(file.Name())
			require.NoError(t, err)

			omit, err := omitter.Omit(resourceList)
			require.NoError(t, err)
			require.Equal(t, tc.omit, omit)
		})
	}

}
