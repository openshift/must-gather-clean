# Replacement canonicalization and validation

Replacement maps for `IP`, `MAC`, and `Domain` are interpreted by obfuscation
type. Percent-encoded input is decoded byte by byte. Every valid `%HH` escape
represents exactly one byte, and hexadecimal digits are case-insensitive.
Decoding happens once: `%25` becomes `%`, while `%2531` remains `%31`.
Malformed escapes such as `%`, `%4`, and `%zz` remain literal.

The canonical IP, MAC, and Domain value is the value written by the current
report writer, not the configured alias spelling. IP canonical values use
uppercase ASCII with `-` and `_` replaced by `.`; IPv6 spelling is not
compressed or expanded. MAC canonical values contain exactly six two-digit
uppercase hexadecimal groups separated by `:`. Domain canonical values are
non-empty, lowercase, printable ASCII. Configured domain names must be
non-empty printable ASCII literals and must not contain regex metacharacters
other than `.`, which the current matcher escapes.

Reports produced by older versions can contain Domain values with uppercase
letters or non-ASCII printable text. The report loader accepts non-empty
printable Unicode spellings and preserves their exact spelling when restoring
data; it does not normalize the reported value.

For example, these IP keys identify the same value:

```yaml
replacement:
  "192.0.2.1": x-ipv4-0000000001-x
  "192-0-2-1": x-ipv4-0000000001-x
  "%31%39%32%2E%30%2E%32%2E%31": x-ipv4-0000000001-x
```

An invalid typed key, two different replacement tokens for one canonical value,
or one non-empty replacement token for different canonical values in the same
replacement map makes configuration loading fail. Trackers created for one
configuration-driven cleaning run share token reservations. A newly
generated token avoids every token imported from configuration or reserved by
another tracker. A canonical value reuses an imported token only when that
association is unambiguous.

Independent replacement maps from historical reports can contain the same token
for different canonical values or different tokens for the same canonical
value. These cross-group conflicts are accepted and marked ambiguous, so new
token generation avoids them and imported tokens for an ambiguous canonical
value are not reused. The first new token generated for that canonical value is
shared by subsequent trackers.
Batch registration is atomic: a conflict within one map leaves the shared
reservation maps unchanged. Keyword, Regex, Exact, AzureResources, and static
replacement outputs do not participate in these reservations.

## Report validation

The `deobfuscate` command restores data from reports. Report IP and MAC values
must use their canonical spelling. Domain values must be non-empty printable
text without spaces; historical spelling, including uppercase and non-ASCII
text, is preserved. This validation runs for those groups even when their
configured replacement map is empty. For reversible entries, report
configuration and occurrence metadata checks run first.

Tracker reports return detached replacement slices and counter maps. Counter
maps are not ordered and the tracker sorts replacements; occurrence
serialization order belongs to the reporting layer and is not part of this
tracker contract. `LoadReport` rejects
configuration and group mismatches and missing occurrence metadata for
reversible groups. It does not reject conflicting token ownership, replacement
chains, or tokens embedded in unsupported outputs; those reverse mappings are
omitted and the affected tokens remain unchanged.

`NewSimpleTrackerMapForObfuscation` rejects typed aliases that canonicalize to
one value but specify different outputs. For reservation enabled trackers it
also rejects one output token assigned to different canonical values within the
same replacement map; the same conflict is accepted as ambiguous between
independent maps. Imported ambiguous associations remain available to their
own trackers for exact replay, but are not reused by generated replacements.
`SimpleTracker.Initialize` panics on a second initialization, on differing
tokens for one canonical value, and, for reservation enabled trackers, on
conflicting token ownership. `LoadReport` does not call `Initialize`.
