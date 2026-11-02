# Keyword matching

Keyword obfuscation treats every replacement key as a literal, case-sensitive
byte string. `Secret` and `secret` are different keywords. Matching does not
decode percent escapes or normalize Unicode.

Each `Path` call receives one complete relative path. Each `Contents` call
receives one line of file content, including its line ending when present, so a
keyword cannot span multiple lines. The cleaner replaces invalid UTF-8 in file
content with `U+FFFD` before keyword matching; `Contents` itself still operates
on arbitrary bytes for callers that invoke it directly.

Each call scans the input once from left to right. At each position, the longest
matching keyword wins. Map iteration order does not affect the transformed
text. After a
match, scanning resumes after the consumed input, so matches never overlap.
Replacement text is copied to the output but is not scanned again during the
same call. A later configured obfuscator still processes that output in the
normal chaining order.

`Path` and `Contents` share the replacement tracker. Counts accumulate across
repeated and concurrent calls. Every configured key is added to the report,
including keys with a zero occurrence count. The empty key is ignored while
matching, but remains in the report as a zero-count entry with an empty
canonical spelling; it should not be configured.
