# Report ordering and publication

Replacement groups remain in configured obfuscator order, while content inside
each group is sorted before serialization:

1. replacements by canonical value, then replacement token;
2. occurrences by original spelling, then count; and
3. omitted paths lexically.

Collection also rejects a mismatch between the configured obfuscator count and
the number of collected replacement groups. The resulting error is returned
when the report is written.

This guarantees a stable serialization for the data collected by the trackers.
Given the same ordered replacement groups and the same replacement and omission
entries, different Go map iteration or insertion orders produce a byte-for-byte
identical YAML report. It does not make replacement assignment deterministic by
itself: consistent tokens are allocated when a worker first encounters a value,
so file traversal order can still associate different tokens with different
values.

## Publication

On Unix-like systems the destination directory is opened and pinned before
encoding. The report is written to a uniquely named temporary file in that
directory and then renamed over the target relative to the pinned directory. A
reader therefore sees either the previous report or the complete new report,
and a failure before the rename leaves the previous report untouched. Go does
not guarantee atomic replacement on every platform supported by the project.
The temporary file is visible in the output directory while the report is being
encoded. Cleanup is best effort, and an interrupted process can leave the
temporary file behind. The write is not synced to durable storage.

Symlink components in the report path and its targets are resolved before the
write, so `report.yaml` continues to point at its target after a relative,
absolute, chained, or dangling target is created. A path whose missing component
is followed by `..`, or whose missing final target is directory-shaped through a
trailing separator or `.`, is rejected instead of being redirected to another
file. Symlink chains must also remain usable by the operating system after the
write. On Unix-like systems, the parent directory of the resolved target must be
readable and searchable so it can be opened and pinned, and writable for
publication. Replacing the directory entry requires directory permissions
instead of the previous target's write permission; owner, group, ACLs, extended
attributes, and hard-link identity are not copied. An existing target is checked
to be a regular file before staging; a concurrent replacement can still be
overwritten by the final rename. Permission and special mode bits are preserved
when it already exists.

New report directories use owner mode `0700`; a filesystem may additionally
inherit special bits such as setgid from the parent directory. This owner access
is established after creation so a restrictive umask cannot prevent publication.
New reports use base mode `0666`, filtered by umask. Existing directories and
files keep their permission and special mode bits. Publication uses paths
relative to the opened directory, so a long target path does not also carry the
temporary file name.
