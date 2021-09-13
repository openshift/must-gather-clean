package obfuscator

import (
	"fmt"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"k8s.io/klog/v2"
)

// generator consists of the required fields for the consistent,static obfuscations and the count of the obfuscations
// This implements the methods static, consistent inorder to return the required replacement based on the replacementType
type generator struct {
	template string
	static   string
	count    int
}

func (g *generator) generateConsistentReplacement() string {
	g.count++
	if g.count > maximumSupportedObfuscations {
		klog.Exitf("maximum number of obfuscations exceeded: %d", maximumSupportedObfuscations)
	}
	r := fmt.Sprintf(g.template, g.count)
	return r
}

func (g *generator) generateStaticReplacement() string {
	return g.static
}

// generateReplacement returns the replacement based on the replacementType argument
func (g *generator) generateReplacement(replacementType schema.ObfuscateReplacementType, key string, tracker ReplacementTracker) string {
	var replacement string
	switch replacementType {
	case schema.ObfuscateReplacementTypeStatic:
		replacement = tracker.GenerateIfAbsent(key, g.generateStaticReplacement)
	case schema.ObfuscateReplacementTypeConsistent:
		replacement = tracker.GenerateIfAbsent(key, g.generateConsistentReplacement)
	}
	return replacement
}

// newGenerator creates a generator objects and populates with the provided arguments
func newGenerator(template, static string) *generator {
	return &generator{template: template, static: static}
}
