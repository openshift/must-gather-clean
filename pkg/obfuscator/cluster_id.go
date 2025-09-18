package obfuscator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

var (
	// From OCM codebase, pkg/models/uid.go
	ocmClusterIdPattern            = "(?:^|[^0-9a-zA-Z])([0-9a-v]{32})(?:[^0-9a-zA-Z]|$)"
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
	matches := c.clusterIdRegex.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			clusterID := match[1] // Extract the captured cluster ID from group 1
			fullMatch := match[0] // The full match including boundary characters
			replacement := c.obfsGenerator.generateReplacement(clusterID, clusterID, 1, c.ReplacementTracker)
			// Replace the full match with the boundary characters + replacement
			output = strings.ReplaceAll(output, fullMatch, strings.Replace(fullMatch, clusterID, replacement, 1))
		}
	}
	return output
}
