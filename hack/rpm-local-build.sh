#!/bin/bash
set -euo pipefail

if [[ ! -d dist/rpmbuild  ]]; then
	echo "Cannot build as artifacts are not generated. Run hack/rpm-prepare.sh first"
	exit 1
fi

top_dir="$PWD/dist/rpmbuild"
echo "Building locally"
rpmbuild --define "_topdir $top_dir" -ba dist/rpmbuild/SPECS/must-gather-clean.spec
