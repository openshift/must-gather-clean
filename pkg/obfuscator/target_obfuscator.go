package obfuscator

import "github.com/openshift/must-gather-clean/pkg/schema"

type targetObfuscator struct {
	target     schema.ObfuscateTarget
	obfuscator ReportingObfuscator
}

func (t *targetObfuscator) Path(s string) string {
	if t.target == schema.ObfuscateTargetAll || t.target == schema.ObfuscateTargetFilePath {
		return t.obfuscator.Path(s)
	}
	return s
}

func (t *targetObfuscator) Contents(s string) string {
	if t.target == schema.ObfuscateTargetAll || t.target == schema.ObfuscateTargetFileContents {
		return t.obfuscator.Contents(s)
	}
	return s
}

func (t *targetObfuscator) Report() ReplacementReport {
	return t.obfuscator.Report()
}

func NewTargetObfuscator(target schema.ObfuscateTarget, obfuscator ReportingObfuscator) ReportingObfuscator {
	return &targetObfuscator{
		target:     target,
		obfuscator: obfuscator,
	}
}
