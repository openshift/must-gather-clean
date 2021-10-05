package obfuscator

import (
	"regexp"
	"strings"

	"github.com/openshift/must-gather-clean/pkg/schema"
	"golang.org/x/crypto/ssh"
)

const (
	// consistentSSHTemplate refers to a static replacement for any identified SSH value
	consistentSSHTemplate          = "x-%010d-x"
	maximumSupportedObfuscationSSH = 9999999999
)

var (
	sshKeyPattern = regexp.MustCompile(`(ssh-rsa AAAAB3NzaC1yc2|ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNT|ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzOD|ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1Mj|ssh-ed25519 AAAAC3NzaC1lZDI1NTE5|ssh-dss AAAAB3NzaC1kc3)[0-9A-Za-z+/]+[=]{0,3}`)
)

type sshObfuscator struct {
	ReplacementTracker
	replacements []replacementGenerator
}

func (o *sshObfuscator) Path(s string) string {
	return o.Contents(s)
}

func (o *sshObfuscator) Contents(s string) string {
	return o.replace(s)
}

func (o *sshObfuscator) replace(input string) string {
	var keyType string
	output := input

	for _, r := range o.replacements {
		matches := r.pattern.FindAllString(input, -1)
		for _, matchedPattern := range matches {
			validSSHkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(matchedPattern))
			if validSSHkey != nil && err == nil {
				// extract the key
				key := strings.Split(matchedPattern, " ")
				keyType = key[len(key)-2]
				if r.generator.replacementType == schema.ObfuscateReplacementTypeStatic {
					s := strings.ReplaceAll(input, matchedPattern, "") + keyType
					return s
				}

				replacement := r.generator.generateReplacement(matchedPattern, input, 1, o.ReplacementTracker)
				output = strings.ReplaceAll(output, matchedPattern, replacement)
			}
		}
	}
	return output
}

func NewSSHObfuscator(replacementType schema.ObfuscateReplacementType, tracker ReplacementTracker) (ReportingObfuscator, error) {
	generator, err := newGenerator(consistentSSHTemplate, "", maximumSupportedObfuscationSSH, replacementType)
	if err != nil {
		return nil, err
	}
	return &sshObfuscator{
		ReplacementTracker: tracker,
		replacements: []replacementGenerator{
			{pattern: sshKeyPattern, generator: generator},
		},
	}, nil
}
