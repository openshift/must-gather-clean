# Matcher behavior and deterministic ordering

This document describes the detection and ordering rules introduced by the
percent-aware matcher changes. These rules are part of the observable behavior
of `must-gather-clean`; tests in `pkg/obfuscator`, `pkg/reporting`, and
`pkg/deobfuscator` define the detailed edge cases.

## Processing model

Each configured obfuscator is a single-pass transformation. Matches are
selected against the input, replacements are generated, and the resulting
replacement text is not scanned again by that same invocation. Later
obfuscators in the configuration still process the output of earlier
obfuscators, as described in the chaining section of the README.

A consequence is that running an already obfuscated string through the same
matcher a second time is not a supported invariant. In particular, replacement
text can add punctuation next to text that was deliberately preserved, which
can change token boundaries on a second pass. The supported invariants are
deterministic selection for a given input and configuration, complete handling
of the forms described below, and stable output/report ordering.

## Percent-encoded input

The built-in IP, MAC, and Domain matchers decode one layer of valid `%HH`
escapes for detection. They replace only the bytes belonging to the detected
value; the encoded context before and after it remains byte-for-byte intact.

For example, with an IP matcher:

```text
value%20%31%39%32%2E%30%2E%32%2E%31%0Aend
```

becomes:

```text
value%20xxx.xxx.xxx.xxx%0Aend
```

Rules for decoding:

* Each valid `%HH` escape represents exactly one byte.
* `%2E` and `%2e` are equivalent to `.`; hexadecimal digits are
  case-insensitive.
* Only one decoding layer is applied. `%2531` represents `%31`, not `1`, and is
  not treated as another IP byte.
* A malformed escape such as `%zz` or a trailing `%` remains literal input.
* A match may begin at `%`, but not between the two hexadecimal digits of one
  escape.
* Encoded and raw spellings of one canonical value share a consistent
  replacement token.

This support applies to values embedded in larger encoded text. It does not
perform recursive URL decoding.

## IP addresses

### IPv4 forms

The matcher accepts four decimal IPv4 groups separated consistently by one of:

* dots, as in `192.0.2.1`;
* hyphens, as in `192-0-2-1`;
* underscores, as in `192_0_2_1`.

Any group may be zero, including the first group. Alternate-separator values
are nevertheless reported as normal dotted IP values. A single value cannot mix
separator families while being parsed as one IPv4 address; mixed forms are
handled only when they are distinct adjacent matches.

The report canonical form is the normalized dotted form produced by
`net.ParseIP`, with IPv6 text uppercased by the common canonicalizer.

### IPv4 numeric-token boundaries

The previous regexp could expose a valid-looking suffix of a malformed numeric
token. The parser now keeps the whole malformed token intact. These inputs are
left unchanged by the IP matcher:

```text
192.0.2.256.7
192.0.2.256.1.2.3
version 0.37.4.10188
timestamp=2021_09_13_15_00_14.222807563
node1.10.0.0.2
```

In `192.0.2.256.1.2.3`, `1.2.3` is not treated as an independently delimited
IPv4 value. The same rule applies to percent-encoded spellings of the token.

Complete addresses that are clearly delimited remain independent. A repeated
separator delimits tokens, so both values in `0.0.0.1..10.0.0.1` are replaced.
Likewise, a complete IPv4 value immediately before a single separating dot does
not make the following IPv4 value a malformed-token suffix; both values in
`0.0.0.1.10.0.0.1` are replaced. A textual label ending in digits still keeps
the address that follows it, as in `node1-10.0.0.1` and `node1_10.0.0.2`.

An underscore or hyphen followed by a complete address is a token boundary even
when the preceding dotted token is malformed. Both values in
`1.2.3.4_5.6.7.8` are replaced, while in `192.0.2.256_4.5.6.7` the malformed
first token is preserved and the second value is replaced. The same boundaries
apply to percent-encoded separators.

### IPv6 forms

IPv6 detection is case-insensitive and accepts complete and compressed forms,
including trailing compression such as `a::` and `0::`. IPv6 values with an
embedded IPv4 tail, such as `64:ff9b::192.0.2.1`, are replaced as one IPv6
value so that no partial IPv6 prefix is left in the output. The embedded IPv4
tail may use dots, underscores, hyphens, or percent-encoded spellings of those
separators.

A short terminal compression after `-`, `_`, or `.` is preserved because it is
normally an identifier component rather than an address. For example,
`foo-a::/64`, `foo-a::`, and `client-ca::/etc/tls/client-ca.crt` remain intact.
The same exception applies to encoded forms such as `foo-a%3A%3A%2F64`.
A short terminal compression followed by a non-CIDR path is also preserved, as
in `/ActiveConnection/1::/run/NetworkManager/...`; a numeric CIDR suffix remains
eligible for masking.

Field labels remain outside the replaced span. For example:

```text
V6OVNMasqueradeIP:fd69::5
```

becomes:

```text
V6OVNMasqueradeIP:xxx:xxx:xxx:xxx:xxx:xxx:xxx:xxx
```

for a static IPv6 replacement. Colon-heavy identifiers that are not addresses,
such as `client-ca::/etc/tls/client-ca.crt`, remain unchanged. The matcher also
does not split a valid-looking suffix out of an invalid run such as
`1:2:3:4:5:6:7:8:9`. In a run containing an embedded IPv4 tail, an invalid
IPv6 prefix is preserved and only the independently recognizable IPv4 tail is
replaced, as in `1:2:3:4:5:6:7:192.0.2.1`.

CPE and CRN values are colon-delimited identifiers. A full-form sequence of
eight decimal fields, such as `cpe:/a:1:2:3:4:5:6:7:8`, remains unchanged.
Compressed or hexadecimal IPv6 values after a textual field are still detected,
as in `crn:v1:bluemix:public:resource:2001:db8::1`.

### Preserved addresses

`127.0.0.1`, `0.0.0.0`, and `::1` are excluded in raw and encoded forms. An
IPv4-mapped IPv6 value whose embedded IPv4 address is `127.0.0.1` is also left
unchanged.

## MAC addresses

A MAC value consists of exactly six groups of two hexadecimal digits. Groups
may use colons or hyphens, and the separators may be mixed. Leading zeroes are
required. Examples:

```text
52:DF:20:08:6C:FF
52-df-20-08-6c-ff
52-DF:20-08:6C-FF
52%3Adf%3A20%3A08%3A6c%3Aff
```

Raw and encoded forms map to the same canonical report value. MAC canonical
values use uppercase hexadecimal digits and colons, for example
`52:DF:20:08:6C:FF`. Matching does not start inside a percent triplet and does
not treat a longer hexadecimal token as a MAC unless six two-digit groups with
separators are present.

## Domain names

Configured domains are literal, case-insensitive strings. They are no longer
used as regular-expression fragments. Matching works on the configured suffix,
so configuring `example.com` replaces both `example.com` and
`sub.example.com`; the subdomain prefix remains in the output. The report's
original occurrence can include the dotted subdomain prefix so that the
mapping retains the spelling relevant to the replacement.

The same matcher recognizes raw and one-layer percent-encoded spellings. For
example, all of these contain the configured value `example.com`:

```text
EXAMPLE.COM
example%2Ecom
%65xample.com
```

Malformed percent escapes remain literal. A configured literal `%` may match
both the raw spelling and its `%25` encoding, but `%42` in the input is not
treated as the literal character `B` for a configured domain containing `%42`.

Configuration validation is stricter than the previous regexp construction:

* an empty domain is rejected;
* non-printable bytes, DEL, and non-ASCII bytes are rejected; use punycode for
  internationalized names;
* the regexp metacharacters `+*?()|[]{}^$\` are rejected because their old
  meaning was ambiguous;
* configurations that equal after ASCII case folding are deduplicated.

Legacy filename-oriented boundary behavior is retained in both `Path` and
`Contents`. A configured value can therefore match inside a longer path label,
as in `notexample.com` becoming `notobfuscated.com`.

## Keywords

Keyword matching is literal and case-sensitive. Within one input, replacement
is non-overlapping and single-pass:

1. the longest keyword at the current input position wins;
2. equal-length keywords are tried in lexical order;
3. after a match, scanning resumes after the matched input bytes;
4. generated replacement text is not scanned again.

The report includes every configured keyword, including keywords with a zero
occurrence count.

## Match selection and replacement order

The matchers use explicit priorities so that overlapping candidates produce
the same result regardless of map iteration or configuration ordering.

### Domain candidates

Overlapping domains are selected using this order:

1. larger legacy specificity, calculated as `len(domain) + number of dots`;
2. earlier textual start;
3. lexical domain value.

The most specific configured domain wins even when a less specific domain
starts earlier. Selected, non-overlapping matches are emitted in ascending
textual start order. A `Path` or `Contents` call therefore advances through the
input once, left to right.

### IP candidates

Overlapping IP candidates are selected using this order:

1. earlier textual start;
2. longer span at the same start;
3. IPv6 before IPv4 at the same span;
4. lexical canonical value as the final tie-breaker.

After selection, matches are emitted in textual order. An IPv6 candidate that
ends inside a valid IPv4 candidate is discarded so that replacing it cannot hide
the IPv4 tail.

### Consistent replacement allocation

For IP, MAC, and Domain consistent replacements, match selection happens before
replacement generation. Selected matches are sorted in textual order before
tokens are requested from the tracker. Consequently, new consistent IDs for one
matcher call follow the order in which the values occur in the input, including
when raw and encoded forms are mixed.

Configured obfuscators are still applied in configuration order, one after the
other, and the report keeps one replacement group per configured obfuscator.

## Tracker and report ordering

The tracker returns copies of replacement counters, so callers cannot mutate
tracker state through a report. Replacement and occurrence ordering is stable:

1. replacement groups follow the configured obfuscator order;
2. replacements within a group are sorted by `Canonical`, then `ReplacedWith`;
3. occurrences within a replacement are sorted by `Original`, then `Count`;
4. omitted paths are sorted lexically before they are reported.

These rules make reports deterministic for completed operations even though
internal lookup maps do not have a defined iteration order.

Report writing is atomic: the YAML is staged in a temporary directory beside
the target and renamed into place. A symlink at the report path is resolved and
the target is updated. New report directories use base mode `0700` filtered by
the process umask; existing directories keep their permission bits. A new
report uses base mode `0666` filtered by umask, while an existing report keeps
its permission bits.

## Reversible reports

Report-driven deobfuscation restores canonical IP, MAC, and Domain values. It
does not preserve the original spelling of every match. For example, encoded
and alternate-separator input can be restored to the canonical value stored in
the report.

The report loader validates replacement-group counts, configuration metadata,
canonical values, duplicate token ownership, and replacement chains. Tokens
that cannot be restored unambiguously are left unchanged. Reports produced
before this behavior was introduced are not migrated; new reports are the
supported input for deobfuscation.

## Permanent parser tests

`pkg/obfuscator/matcher_property_test.go` adds deterministic property tests and
permanent fuzz targets. The deterministic tests generate IPv4, MAC, and domain
spellings and assert that valid forms are replaced and canonical reports are
produced. The IPv4 fuzz targets check accepted forms and malformed numeric
suffixes. The arbitrary-input fuzz target checks panic resistance and
deterministic output/report results for IP, MAC, and Domain matchers.

Run the regular property suite with the rest of the package tests:

```sh
go test -mod=vendor ./pkg/obfuscator
```

Run each fuzz target for a bounded interval, for example:

```sh
go test -mod=vendor -run '^$' -fuzz '^FuzzMatcherArbitraryInput$' -fuzztime=30s ./pkg/obfuscator
go test -mod=vendor -run '^$' -fuzz '^FuzzIPMatcherValidIPv4Forms$' -fuzztime=30s ./pkg/obfuscator
go test -mod=vendor -run '^$' -fuzz '^FuzzIPMatcherRejectsInvalidNumericSuffix$' -fuzztime=30s ./pkg/obfuscator
```

If a target finds a counterexample, Go writes it under
`pkg/obfuscator/testdata/fuzz/` so that the normal package test suite replays
the regression from then on.
