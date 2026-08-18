#!/usr/bin/env bash
#
# Copyright 2026 Chengxi Luo
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Checks that every published manifest carries the version being released.
#
# The publishes are irreversible and independent: nothing stops five registries
# from accepting 0.2.0 and the sixth from shipping a stale 0.1.0, and no
# registry lets you replace a published version afterwards. So this runs before
# any of them, and a mismatch fails the release instead of half-completing it.
#
#   scripts/check-versions.sh v0.2.0

set -euo pipefail

version="${1:-}"
if [ -z "$version" ]; then
	echo "usage: $(basename "$0") <version>   (leading v optional)" >&2
	exit 2
fi
version="${version#v}"

cd "$(git rev-parse --show-toplevel)"

fail=0
report() { # report <label> <found>
	if [ "${2:-}" != "$version" ]; then
		printf '  %-44s %s\n' "$1" "${2:-<no version found>}"
		fail=1
	fi
}

# The published manifests.
report "lib/rust/serify/Cargo.toml" \
	"$(sed -n 's/^version = "\(.*\)"/\1/p' lib/rust/serify/Cargo.toml | head -1)"
report "lib/rust/derive/Cargo.toml" \
	"$(sed -n 's/^version = "\(.*\)"/\1/p' lib/rust/derive/Cargo.toml | head -1)"
report "lib/python/pyproject.toml" \
	"$(sed -n 's/^version = "\(.*\)"/\1/p' lib/python/pyproject.toml | head -1)"
report "package.json" \
	"$(sed -n 's/.*"version": "\([^"]*\)".*/\1/p' package.json | head -1)"
report "lib/csharp/Serify.csproj" \
	"$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' lib/csharp/Serify.csproj | head -1)"
report "lib/elixir/mix.exs" \
	"$(sed -n 's/.*version: "\([^"]*\)".*/\1/p' lib/elixir/mix.exs | head -1)"
# The first <version> in a pom is the project's own; <modelVersion> does not
# match, being a different tag.
report "lib/java/pom.xml" \
	"$(grep -o '<version>[^<]*</version>' lib/java/pom.xml | head -1 | sed 's/<[^>]*>//g')"

# Every pom that depends on the Java library by version. These are not
# published, but a stale one breaks the build of the workers the conformance
# suite runs, which is how the mismatch would otherwise be found: late.
dep_version() { # dep_version <pom>
	tr -d ' \n\t' <"$1" |
		grep -o '<artifactId>serify</artifactId><version>[^<]*' |
		head -1 | sed 's/.*<version>//'
}
for pom in examples/java/pom.xml test/cases/*/java/pom.xml; do
	report "$pom (serify dependency)" "$(dep_version "$pom")"
done

# The Rust library depends on its own derive macro by version, and crates.io
# resolves that against the index, not the path.
report "lib/rust/serify/Cargo.toml (serify-derive dep)" \
	"$(sed -n 's/^serify-derive = { version = "\([^"]*\)".*/\1/p' lib/rust/serify/Cargo.toml | head -1)"

if [ "$fail" -ne 0 ]; then
	echo >&2
	echo "version mismatch: the manifests above do not say $version" >&2
	exit 1
fi

echo "all manifests carry $version"
