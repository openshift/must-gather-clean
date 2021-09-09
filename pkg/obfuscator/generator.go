package obfuscator

import (
	"fmt"

	"k8s.io/klog/v2"
)

// generator consists of the required fields for the consistent,static obfuscations and the count of the obfuscations
// This implements the methods static, consistent inorder to return the required replacement based on the replacementType
// This implementation is intentionally not thread-safe, but should be used under the locking of ReplacementTracker.GenerateIfAbsent to ensure
// the results are correct across goroutines.
type generator struct {
	template string
	static   string
	count    int
	max      int
	exitFunc func(string, int)
}

func (g *generator) generateConsistentReplacement() string {
	g.count++
	if g.count > g.max {
		g.exitFunc(g.template, g.max)
		return ""
	}
	r := fmt.Sprintf(g.template, g.count)
	return r
}

func (g *generator) generateStaticReplacement() string {
	return g.static
}

// newGenerator creates a generator objects and populates with the provided arguments
func newGenerator(template, static string, maxSupported int) *generator {
	return &generator{template: template, static: static, max: maxSupported, exitFunc: func(t string, m int) {
		// we exit here since this is an error we can't possibly recover from automatically
		klog.Exitf("Please review your configuration, maximum number of obfuscations was exceeded: %d for template: %s", m, t)
	}}
}
