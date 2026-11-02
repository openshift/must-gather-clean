package obfuscator

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/must-gather-clean/pkg/schema"
)

func TestDomainObfuscatorContents(t *testing.T) {
	for _, tc := range []struct {
		name    string
		domains []string
		input   []string
		output  []string
		report  ReplacementReport
	}{
		{
			name:    "basic",
			domains: []string{"redhat.com", "openshift.com", "okd.io"},
			input: []string{
				"received request on openshift.com",
				"received request on https://docs.okd.io",
			},
			output: []string{
				"received request on domain0000000001",
				"received request on https://docs.domain0000000002",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "openshift.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"openshift.com": uint(1),
				}},
				{Canonical: "okd.io", ReplacedWith: "domain0000000002", Counter: map[string]uint{
					"docs.okd.io": uint(1),
				}},
			}},
		},
		{
			name: "subdomains",
			domains: []string{
				"docs.okd.io",
				"cloud.redhat.com",
			},
			input: []string{
				"okd.io",
				"docs.okd.io",
				"cloud.redhat.com",
				"beta.cloud.redhat.com",
			},
			output: []string{
				"okd.io",
				"domain0000000001",
				"domain0000000002",
				"beta.domain0000000002",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "docs.okd.io", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"docs.okd.io": uint(1),
				}},
				{Canonical: "cloud.redhat.com", ReplacedWith: "domain0000000002",
					Counter: map[string]uint{
						"cloud.redhat.com":      uint(1),
						"beta.cloud.redhat.com": uint(1),
					},
				}}},
		},
		{
			name:    "multiple matches per line",
			domains: []string{"redhat.com", "openshift.com"},
			input: []string{
				"connect to api.openshift.com and sso.redhat.com and console.openshift.com",
			},
			output: []string{
				"connect to api.domain0000000001 and sso.domain0000000002 and console.domain0000000001",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "openshift.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"api.openshift.com":     uint(1),
					"console.openshift.com": uint(1),
				}},
				{Canonical: "redhat.com", ReplacedWith: "domain0000000002", Counter: map[string]uint{
					"sso.redhat.com": uint(1),
				}},
			}},
		},
		{
			name:    "multi-level subdomains",
			domains: []string{"test.com", "test.info", "test.org"},
			input: []string{
				"received request on abc.test.com",
				"received request on def.test.info",
				"received request on ghi.abc.test.com",
				"received request on pqr.ghi.abc.test.com",
			},
			output: []string{
				"received request on abc.domain0000000001",
				"received request on def.domain0000000002",
				"received request on ghi.abc.domain0000000001",
				"received request on pqr.ghi.abc.domain0000000001",
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "test.com", ReplacedWith: "domain0000000001",
					Counter: map[string]uint{
						"abc.test.com":         uint(1),
						"ghi.abc.test.com":     uint(1),
						"pqr.ghi.abc.test.com": uint(1),
					},
				},
				{Canonical: "test.info", ReplacedWith: "domain0000000002",
					Counter: map[string]uint{
						"def.test.info": uint(1),
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			for idx, i := range tc.input {
				output := o.Contents(i)
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestDomainObfuscatorMatchesCaseInsensitiveValues(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"example.com"},
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	assert.Contains(t, obfuscator.Contents("host=EXAMPLE.com"), "domain0000000001")
	assert.Contains(t, obfuscator.Contents("host=secret%2EEXAMPLE%2ECOM"), "domain0000000001")
}

func TestDomainObfuscatorReplacesPercentEncodedDomains(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"example.com"},
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	output := obfuscator.Contents("host=secret%2Eexample%2Ecom")

	assert.Equal(t, "host=secret%2Edomain0000000001", output)
	assert.Equal(t, "host=domain0000000001", obfuscator.Contents("host=%65xample%2Ecom"))
	assert.Equal(t, "host=%zzdomain0000000001", obfuscator.Contents("host=%zzexample.com"))
}

func TestDomainObfuscatorReplacesNestedConfiguredValuesBeforeOverlappingPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "earlier domain followed by a longer configured suffix",
			input:  "xexample.com.corp.net",
			output: "xdomain0000000001.domain0000000002",
		},
		{
			name:   "preserved subdomain",
			input:  "sub.example.com.corp.net",
			output: "sub.domain0000000001.domain0000000002",
		},
		{
			name:   "two domains",
			input:  "example.com.corp.net",
			output: "domain0000000001.domain0000000002",
		},
		{
			name:   "case insensitive",
			input:  "EXAMPLE.COM.CORP.NET",
			output: "domain0000000001.domain0000000002",
		},
		{
			name:   "encoded separator and domain",
			input:  "xexample%2Ecom.corp%2Enet",
			output: "xdomain0000000001.domain0000000002",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewDomainObfuscator(
				[]string{"example.com", "corp.net"},
				schema.ObfuscateReplacementTypeConsistent,
				NewSimpleTracker(),
			)
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
			assert.NotContains(t, obfuscator.Contents(tc.input), "example.com")
			assert.NotContains(t, obfuscator.Contents(tc.input), "corp.net")
			assert.Equal(t, tc.output, obfuscator.Path(tc.input))
		})
	}
}

func BenchmarkDomainObfuscatorHighCardinality(b *testing.B) {
	const domainCount = 1_000
	domains := make([]string, domainCount)
	for index := range domains {
		domains[index] = "cluster-" + strconv.Itoa(index) + ".example.com"
	}

	inputBuilder := strings.Builder{}
	for index, domain := range domains {
		if index > 0 {
			inputBuilder.WriteByte(' ')
		}
		inputBuilder.WriteString("host-" + strconv.Itoa(index) + "." + domain)
	}
	input := inputBuilder.String()

	obfuscator, err := NewDomainObfuscator(
		domains,
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(b, err)

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if output := obfuscator.Contents(input); strings.Contains(output, "example.com") {
			b.Fatal("unreplaced domain")
		}
	}
}

func TestDomainObfuscator_FileName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		output  string
		domains []string
		report  ReplacementReport
	}{
		{
			name:    "domain with extension",
			domains: []string{"test.com"},
			input:   "requests.test.com.log",
			output:  "requests.domain0000000001.log",
			report: ReplacementReport{[]Replacement{
				{Canonical: "test.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"requests.test.com": uint(1),
				}},
			}},
		},
		{
			name:    "non-matching domain",
			domains: []string{"test.com"},
			input:   "report.test",
			output:  "report.test",
			report:  ReplacementReport{[]Replacement{}},
		},
		{
			name: "overlapping domains",
			domains: []string{
				"openshift.com",
				"devcluster.openshift.com",
			},
			input:  "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.qe.devcluster.openshift.com/installer/installer/logs",
			output: "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.qe.domain0000000001/installer/installer/logs",
			report: ReplacementReport{[]Replacement{
				{Canonical: "devcluster.openshift.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"dualstack.qe.devcluster.openshift.com": uint(1),
				}},
			}},
		},
		{
			name: "overlapping domains flipped",
			domains: []string{
				"devcluster.openshift.com",
				"openshift.com",
			},
			input:  "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.qe.devcluster.openshift.com/installer/installer/logs",
			output: "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.qe.domain0000000001/installer/installer/logs",
			report: ReplacementReport{[]Replacement{
				{Canonical: "devcluster.openshift.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"dualstack.qe.devcluster.openshift.com": uint(1),
				}},
			}},
		},
		{
			name: "overlapping domains flipped and mixed",
			domains: []string{
				"devcluster.openshift.com",
				"openshift.com",
				"qe.devcluster.openshift.com",
			},
			input:  "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.qe.devcluster.openshift.com/installer/installer/logs",
			output: "must-gather-output/namespaces/openshift-kube-apiserver/pods/installer-13-master-02.pamoedo-dualstack.domain0000000001/installer/installer/logs",
			report: ReplacementReport{[]Replacement{
				{Canonical: "qe.devcluster.openshift.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
					"dualstack.qe.devcluster.openshift.com": uint(1),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
			output := o.Path(tc.input)
			assert.Equal(t, tc.output, output)
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestDomainObfuscatorPathPreservesLegacyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		domains []string
		input   string
		output  string
		report  ReplacementReport
	}{
		{
			name:    "domain after a hyphen",
			domains: []string{"f.g.h"},
			input:   "abcde-f.g.h",
			output:  "abcde-obfuscated.com",
			report: ReplacementReport{[]Replacement{
				{Canonical: "f.g.h", ReplacedWith: staticDomainReplacement, Counter: map[string]uint{
					"f.g.h": 1,
				}},
			}},
		},
		{
			name:    "domain after an underscore",
			domains: []string{"f.g.h"},
			input:   "abcde_f.g.h",
			output:  "abcde_obfuscated.com",
			report: ReplacementReport{[]Replacement{
				{Canonical: "f.g.h", ReplacedWith: staticDomainReplacement, Counter: map[string]uint{
					"f.g.h": 1,
				}},
			}},
		},
		{
			name:    "domain embedded in a longer path label",
			domains: []string{"example.com"},
			input:   "notexample.com",
			output:  "notobfuscated.com",
			report: ReplacementReport{[]Replacement{
				{Canonical: "example.com", ReplacedWith: staticDomainReplacement, Counter: map[string]uint{
					"example.com": 1,
				}},
			}},
		},
		{
			name:    "path prefix stops at a hyphen",
			domains: []string{"example.com"},
			input:   "foo-bar.example.com",
			output:  "foo-bar.obfuscated.com",
			report: ReplacementReport{[]Replacement{
				{Canonical: "example.com", ReplacedWith: staticDomainReplacement, Counter: map[string]uint{
					"bar.example.com": 1,
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Path(tc.input))
			replacementReportsMatch(t, tc.report, obfuscator.Report())
		})
	}
}

func TestDomainObfuscatorDeduplicatesCaseInsensitiveConfiguration(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"EXAMPLE.com", "example.com"},
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	assert.Equal(t, "domain0000000001", obfuscator.Contents("EXAMPLE.com"))
	assert.Equal(t, "domain0000000001", obfuscator.Contents("example.com"))
	assert.Equal(t, ReplacementReport{[]Replacement{
		{Canonical: "example.com", ReplacedWith: "domain0000000001", Counter: map[string]uint{
			"EXAMPLE.com": 1,
			"example.com": 1,
		}},
	}}, obfuscator.Report())
}

func TestDomainObfuscatorPreservesLegacyContentBoundaries(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"test.com"},
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	assert.Equal(t, "nodomain0000000001", obfuscator.Contents("notest.com"))
	assert.Equal(t, "domain0000000001.attacker.net", obfuscator.Contents("test.com.attacker.net"))
	assert.Equal(t, "sub.domain0000000001", obfuscator.Contents("sub.test.com"))
	assert.Equal(t, "domain0000000001.", obfuscator.Contents("test.com."))
	assert.Equal(t, "value%20domain0000000001%0Aend", obfuscator.Contents("value%20test.com%0Aend"))

	// Path and Contents share one replacement tracker. Keeping these suffix
	// forms in Contents prevents a path replacement from exposing the original
	// domain in report-driven output verification.
	assert.Equal(t, "host.domain0000000001_uuid", obfuscator.Contents("host.test.com_uuid"))
	assert.Equal(t, "host.domain0000000001.key", obfuscator.Contents("host.test.com.key"))
	assert.Equal(t, "host.domain0000000001.16a4b5b76e9b223b", obfuscator.Contents("host.test.com.16a4b5b76e9b223b"))
}

func TestDomainObfuscatorAcceptsUnderscoreAndLegacyDomainNames(t *testing.T) {
	for _, domain := range []string{
		"_dmarc.example.com",
		"_ldap._tcp.example.com",
		".example.com",
		"example..com",
		"example.com.",
		"example-.com",
		"example._com",
	} {
		t.Run(domain, func(t *testing.T) {
			_, err := NewDomainObfuscator([]string{domain}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)
		})
	}
}

func TestDomainObfuscatorRawAndEncodedFormsShareReplacement(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"example.com"},
		schema.ObfuscateReplacementTypeConsistent,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	forms := []string{
		"sub.example.com",
		"SUB.EXAMPLE.COM",
		"sub%2Eexample%2Ecom",
		"%73ub.example.com",
	}
	output := obfuscator.Contents(strings.Join(forms, " | "))

	reports := obfuscator.Report().Replacements
	require.Len(t, reports, 1)
	assert.Equal(t, "example.com", reports[0].Canonical)
	assert.Equal(t, len(forms), strings.Count(output, reports[0].ReplacedWith))
	for _, form := range forms {
		assert.Equal(t, uint(1), reports[0].Counter[form])
	}
}

func TestBadDomainInput(t *testing.T) {
	_, err := NewDomainObfuscator([]string{"[mustgather.com"}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate regex for domain [mustgather.com")
}

func TestDomainObfuscatorRejectsRegexMetacharacterConfiguration(t *testing.T) {
	for _, domain := range []string{
		"example|mustgather.com",
		"example(example).com",
		"example+.com",
		"example[foo].com",
		"example\\|mustgather.com",
		"example$.com",
	} {
		t.Run(domain, func(t *testing.T) {
			_, err := NewDomainObfuscator([]string{domain}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot contain regex metacharacters")
		})
	}
}

func TestDomainObfuscatorRejectsEmptyDomain(t *testing.T) {
	_, err := NewDomainObfuscator([]string{""}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain cannot be empty")
}

func TestDomainObfuscatorRejectsNonPrintableDomain(t *testing.T) {
	for _, domain := range []string{"cluster example", "cluster\texample", "cluster\nexample", "cluster\x01example"} {
		t.Run(domain, func(t *testing.T) {
			_, err := NewDomainObfuscator([]string{domain}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "domain must be printable ASCII")
		})
	}
}

func TestDomainObfuscatorDoesNotStartInsidePercentEncoding(t *testing.T) {
	obfuscator, err := NewDomainObfuscator([]string{"41example.com"}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "%41example.com"
	assert.Equal(t, input, obfuscator.Contents(input))
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestDomainObfuscatorMatchesLiteralPercentInConfiguredDomain(t *testing.T) {
	for _, tc := range []struct {
		domain string
		inputs []string
	}{
		{
			domain: "example%42.com",
			inputs: []string{
				"example%42.com",
				"example%2542.com",
			},
		},
		{
			domain: "%42.example.com",
			inputs: []string{
				"%42.example.com",
				"%2542.example.com",
			},
		},
		{
			domain: "%25.example.com",
			inputs: []string{
				"%25.example.com",
				"%2525.example.com",
			},
		},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			obfuscator, err := NewDomainObfuscator([]string{tc.domain}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
			require.NoError(t, err)

			for _, input := range tc.inputs {
				assert.Equal(t, "domain0000000001", obfuscator.Contents(input))
			}
			reports := obfuscator.Report().Replacements
			require.Len(t, reports, 1)
			assert.Equal(t, tc.domain, reports[0].Canonical)
			for _, input := range tc.inputs {
				assert.Equal(t, uint(1), reports[0].Counter[input])
			}
		})
	}
}

func TestDomainObfuscatorTreatsMalformedPercentAsLiteral(t *testing.T) {
	obfuscator, err := NewDomainObfuscator(
		[]string{"%zz.example.com"},
		schema.ObfuscateReplacementTypeStatic,
		NewSimpleTracker(),
	)
	require.NoError(t, err)

	assert.Equal(t, staticDomainReplacement, obfuscator.Contents("%zz.example.com"))
	assert.Equal(t, staticDomainReplacement, obfuscator.Path("%zz.example.com"))
}

func TestDomainObfuscatorDoesNotDecodeLiteralPercentAsAnotherByte(t *testing.T) {
	obfuscator, err := NewDomainObfuscator([]string{"example%42.com"}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	require.NoError(t, err)

	input := "exampleB.com"
	assert.Equal(t, input, obfuscator.Contents(input))
	assert.Empty(t, obfuscator.Report().Replacements)
}

func TestNoDomainInput(t *testing.T) {
	_, err := NewDomainObfuscator([]string{}, schema.ObfuscateReplacementTypeConsistent, NewSimpleTracker())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no domainNames supplied for the obfuscation type: Domain")
}

func TestDomainObfuscationStatic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   []string
		output  []string
		domains []string
		report  ReplacementReport
		usePath bool
	}{
		{
			// These are the test cases with the static domain obfuscation.
			name:    "Domain with extension",
			domains: []string{"test.com"},
			input:   []string{"requests.test.com.log"},
			output:  []string{"requests." + staticDomainReplacement + ".log"},
			usePath: true,
			report: ReplacementReport{[]Replacement{
				{Canonical: "test.com", ReplacedWith: staticDomainReplacement,
					Counter: map[string]uint{
						"requests.test.com": uint(1),
					}},
			}},
		},
		{
			name:    "non-matching domain",
			domains: []string{"test.com"},
			input:   []string{"report.test"},
			output:  []string{"report.test"},
			report:  ReplacementReport{[]Replacement{}},
		},
		{
			name:    "Multiple-Matching Domains",
			domains: []string{"test.com"},
			input: []string{
				"The first domain is report.test.com and the second domain is example-test.com",
			},
			output: []string{
				"The first domain is report." + staticDomainReplacement + " and the second domain is example-" + staticDomainReplacement,
			},
			report: ReplacementReport{[]Replacement{
				{Canonical: "test.com", ReplacedWith: staticDomainReplacement,
					Counter: map[string]uint{
						"report.test.com": uint(1),
						"test.com":        uint(1),
					}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewDomainObfuscator(tc.domains, schema.ObfuscateReplacementTypeStatic, NewSimpleTracker())
			require.NoError(t, err)
			for idx, i := range tc.input {
				var output string
				if tc.usePath {
					output = o.Path(i)
				} else {
					output = o.Contents(i)
				}
				assert.Equal(t, tc.output[idx], output)
			}
			replacementReportsMatch(t, tc.report, o.Report())
		})
	}
}

func TestDomainObfuscatorPrefersGloballyMoreSpecificOverlappingDomain(t *testing.T) {
	for _, tc := range []struct {
		name    string
		domains []string
		input   string
		output  string
	}{
		{
			name:    "later domain is longer",
			domains: []string{"ab.cd", "b.cd.ef"},
			input:   "ab.cd.ef",
			output:  "a" + staticDomainReplacement,
		},
		{
			name:    "later domain is longer without dots in overlap",
			domains: []string{"example", "ample.com"},
			input:   "example.com",
			output:  "ex" + staticDomainReplacement,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obfuscator, err := NewDomainObfuscator(
				tc.domains,
				schema.ObfuscateReplacementTypeStatic,
				NewSimpleTracker(),
			)
			require.NoError(t, err)

			assert.Equal(t, tc.output, obfuscator.Contents(tc.input))
		})
	}
}
