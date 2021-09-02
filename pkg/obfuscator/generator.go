package obfuscator

import (
	"fmt"

	"k8s.io/klog/v2"
)

// generator consists of the required fields for the consistent,static obfuscations and the count of the obfuscations
// This implements the methods static, consistent inorder to return the required replacement based on the replacementType
type generator struct {
	template string
	static   string
	count    int
}

func (g *generator) generateConsistentReplacement(_ string) string {
	g.count++
	if g.count > maximumSupportedObfuscations {
		klog.Exitf("maximum number of obfuscations exceeded: %d", maximumSupportedObfuscations)
	}
	r := fmt.Sprintf(g.template, g.count)
	return r
}

func (g *generator) generateStaticReplacement(_ string) string {
	return g.static
}

// NewGenerator creates a generator objects and populates with the provided arguments
func NewGenerator(template, static string, count int) *generator {
	return &generator{template, static, count}
}
