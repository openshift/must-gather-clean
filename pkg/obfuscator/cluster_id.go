package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

var (
	// From OCM codebase, pkg/models/uid.go
	ocmClusterIdPattern            = "[0123456789abcdefghijklmnopqrstuv]{32}"
	clusterIDReplacement           = "x-obfuscated-clusterid-%007d-x"
	staticClusterIDReplacement     = "x-obfuscated-clusterid-aaaaaaa-x"
	maximumSupportedObfuscationIDs = 99999999
)

type clusterIDObfuscator struct {
	ReplacementTracker
	obfsGenerator  generator
	clusterIdRegex *regexp.Regexp
}

func NewClusterIDObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	gen, err := newGenerator(clusterIDReplacement, staticClusterIDReplacement, maximumSupportedObfuscationIDs, replacementType)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator for ClusterID obfuscator: %w", err)
	}

	clusterIdRegex, err := regexp.Compile(ocmClusterIdPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex for ClusterID obfuscator: %w", err)
	}
	return &clusterIDObfuscator{
		ReplacementTracker: tracker,
		obfsGenerator:      *gen,
		clusterIdRegex:     clusterIdRegex,
	}, nil
}

func (c *clusterIDObfuscator) Path(s string) string {
	return c.replaceClusterIDs(s)
}

func (c *clusterIDObfuscator) Contents(s string) string {
	return c.replaceClusterIDs(s)
}

func (c *clusterIDObfuscator) replaceClusterIDs(input string) string {
	output := input
	matches := c.clusterIdRegex.FindAllString(input, -1)
	for _, m := range matches {
		replacement := c.obfsGenerator.generateReplacement(m, m, 1, c.ReplacementTracker)
		output = strings.ReplaceAll(output, m, replacement)
	}
	return output
}
