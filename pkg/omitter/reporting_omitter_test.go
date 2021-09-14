package omitter

import (
	"testing"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOmitPathSingle(t *testing.T) {
	omitter := NewMultiReportingOmitter([]FileOmitter{testingFileOmitterWithPattern(t, "*.log")}, []KubernetesResourceOmitter{})

	omit, err := omitter.OmitPath("some.log")
	require.NoError(t, err)
	assert.True(t, omit, "some.log should be omitted")

	omit, err = omitter.OmitPath("some.dog")
	require.NoError(t, err)
	assert.False(t, omit, "some.dog should not be omitted")

	assert.Equal(t, []string{"some.log"}, omitter.Report())
}

func TestOmitPathMulti(t *testing.T) {
	omitter := NewMultiReportingOmitter([]FileOmitter{
		testingFileOmitterWithPattern(t, "something/not/quite/*/log"),
		testingFileOmitterWithPattern(t, "something/not/quite/b/*"),
	}, []KubernetesResourceOmitter{})

	omit, err := omitter.OmitPath("something/not/quite/a/log")
	require.NoError(t, err)
	assert.True(t, omit, "\"something/not/quite/a/log\" should be omitted")

	omit, err = omitter.OmitPath("something/not/quite/b/anything")
	require.NoError(t, err)
	assert.True(t, omit, "\"something/not/quite/b/anything\" should be omitted")

	omit, err = omitter.OmitPath("some.dog")
	require.NoError(t, err)
	assert.False(t, omit, "some.dog should not be omitted")

	omit, err = omitter.OmitPath("something/not/quite/*/logs")
	require.NoError(t, err)
	assert.False(t, omit, "\"something/not/quite/*/logs\" should not be omitted")
	assert.Equal(t, []string{"something/not/quite/a/log", "something/not/quite/b/anything"}, omitter.Report())
}

func TestOmitK8s(t *testing.T) {
	omitter := NewMultiReportingOmitter([]FileOmitter{}, []KubernetesResourceOmitter{testingK8sResourceOmitter(t)})

	omit, err := omitter.OmitKubeResource(&kube.ResourceListWithPath{
		ResourceList: kube.ResourceList{
			Items: []kube.Resource{
				{
					ApiVersion: "v1",
					Kind:       "kind",
				},
			},
		},
		Path: "some.path",
	})

	require.NoError(t, err)
	assert.True(t, omit, "v1 resource should be omitted")

	omit, err = omitter.OmitKubeResource(&kube.ResourceListWithPath{
		ResourceList: kube.ResourceList{
			Items: []kube.Resource{
				{
					ApiVersion: "v2",
					Kind:       "kind",
				},
			},
		},
		Path: "some.other.path",
	})

	require.NoError(t, err)
	assert.False(t, omit, "v2 resource should not be omitted")

	assert.Equal(t, []string{"some.path"}, omitter.Report())
}

func testingFileOmitterWithPattern(t *testing.T, pattern string) FileOmitter {
	omitter, err := NewFilenamePatternOmitter(pattern)
	require.NoError(t, err)
	return omitter
}

func testingK8sResourceOmitter(t *testing.T) KubernetesResourceOmitter {
	v1 := "v1"
	rKind := "kind"
	omitter, err := NewKubernetesResourceOmitter(&v1, &rKind, []string{})
	require.NoError(t, err)
	return omitter
}
