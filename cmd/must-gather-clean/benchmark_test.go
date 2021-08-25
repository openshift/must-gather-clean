package main

import (
	"runtime"
	"testing"

	"github.com/openshift/must-gather-clean/pkg/cli"
	"github.com/stretchr/testify/require"
)

// BenchmarkBigMustGathers is used to benchmark and profile big must-gathers using the default obfuscation config.
func BenchmarkBigMustGathers(b *testing.B) {
	err := cli.Run(
		"examples/openshift_default.yaml",
		"mgs/must-gather-big.aws",
		"mgs/must-gather.aws.cleaned",
		true,
		".",
		runtime.NumCPU())
	require.Nil(b, err)
}
