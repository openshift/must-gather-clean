package obfuscator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSkipListWord(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected bool
	}{
		{"service", true},
		{"cluster", true},
		{"proxy", true},
		{"Service", false},  // case-sensitive — uppercase not in list
		{"my-service", false},
		{"customname", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.expected, isSkipListWord(tc.input))
		})
	}
}

func TestShouldSkipFreeTextReplacement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		expected bool
	}{
		// Too short (< 5 chars)
		{"short: vm", "vm", true},
		{"short: rg", "rg", true},
		// Plain word — single-case, letters-only (covers all genericSkipWords)
		{"plain lowercase: service", "service", true},
		{"plain uppercase: PROXY", "PROXY", true},
		{"plain uppercase: GPU", "GPU", true},
		// Known infra components
		{"infra: cloud-controller-manager", "cloud-controller-manager", true},
		{"infra: disk-csi-driver", "disk-csi-driver", true},
		{"infra: image-registry", "image-registry", true},
		{"infra: control-plane", "control-plane", true},
		// Semantic versions
		{"semver: 4.19.18", "4.19.18", true},
		{"semver: 1.2.3", "1.2.3", true},
		{"semver: v1.28.5", "v1.28.5", true},
		{"semver: 4.19.20-rc1", "4.19.20-rc1", true},
		// Boundary: exactly 4 chars (too short)
		{"boundary: 4 chars", "abcd", true},
		// Boundary: exactly 5 chars plain word (still skipped — plain word guard)
		{"boundary: 5 chars plain", "hello", true},
		// Boundary: exactly 5 chars mixed case (valid resource name)
		{"boundary: 5 chars mixed case", "Hello", false},
		// Should NOT skip — valid resource names
		{"mixed case: Service", "Service", false},
		{"hyphenated: my-cluster", "my-cluster", false},
		{"with digit: proxy1", "proxy1", false},
		{"camelCase: MyDisk", "MyDisk", false},
		// These were once in knownInfraComponents but are not standard enough
		// for an upstream tool — they must now be treated as replaceable resource names.
		{"formerly-private: cluster-service", "cluster-service", false},
		{"formerly-private: credential-refresher", "credential-refresher", false},
		{"formerly-private: secret-sync-controller", "secret-sync-controller", false},
		{"formerly-private: route-monitor-operator", "route-monitor-operator", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldSkipFreeTextReplacement(tc.input, false))
		})
	}

	// foundViaPhase1=true: isPlainWord guard is bypassed, but other guards remain.
	for _, tc := range []struct {
		name     string
		input    string
		expected bool
	}{
		// Plain lowercase bypassed by Phase 1 → should NOT skip
		{"phase1 plain: johndoe", "johndoe", false},
		{"phase1 plain: customer", "customer", false},
		// genericSkipWords always block, even Phase 1 discoveries
		{"phase1 genericSkip: service", "service", true},
		{"phase1 genericSkip: proxy", "proxy", true},
		// knownInfraComponents always block, even Phase 1 discoveries
		{"phase1 infra: disk-csi-driver", "disk-csi-driver", true},
		{"phase1 infra: image-registry", "image-registry", true},
		// Too short — always block
		{"phase1 short: vm", "vm", true},
		// semver — always block
		{"phase1 semver: 4.19.18", "4.19.18", true},
		// Mixed case — never blocked by isPlainWord anyway
		{"phase1 mixed: Service", "Service", false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldSkipFreeTextReplacement(tc.input, true))
		})
	}
}

func TestIsProtectedContext(t *testing.T) {
	for _, tc := range []struct {
		name     string
		text     string
		start    int
		end      int
		expected bool
	}{
		// Dotted compounds
		{"systemd unit", "containerd.service is running", 11, 18, true},
		{"CRD name", "destinationrules.networking.istio.io", 17, 27, true},

		// Hyphenated compounds
		{"kube compound", "kube-apiserver started", 5, 14, true},
		{"credential-refresher", "msi-credential-refresher ns", 4, 14, true},
		{"service-worker", "service-worker pool", 0, 7, true},
		{"clusters-service", "clusters-service running", 9, 16, true},

		// Exact match of entire compound — not protected (strict subset only)
		{"exact match hyphen", "kube-proxy running", 0, 10, false},
		{"exact match dot", "containerd.service ok", 0, 18, false},

		// Overlapping hyphenated+dotted compound
		{"dotted-hyphenated overlap: apiserver in kube-apiserver.service", "kube-apiserver.service is running", 5, 14, true},
		{"dotted-hyphenated overlap: service in kube-apiserver.service", "kube-apiserver.service is running", 15, 22, true},

		// Standalone word — not protected
		{"standalone", "apiserver started", 0, 9, false},
		{"standalone service", "the service is up", 4, 11, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ranges := findProtectedRanges(tc.text)
			require.Equal(t, tc.expected, isProtectedByRanges(ranges, tc.start, tc.end))
		})
	}
}
