#!/usr/bin/env bash
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
#
# Regenerate the checked-in .schemas/ JSON Schema files by running
# `serify schema` on every case directory in the repo.
#
#   scripts/gen-schemas.sh           regenerate in place (also run by the
#                                    pre-commit hook, on every commit)
#   scripts/gen-schemas.sh --check   regenerate, then fail if anything changed
#                                    (for CI: proves the committed schemas are
#                                    in sync with the schema: sections)

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Every case directory whose schemas are checked in. Add new suites here.
#
# test/cases/invalid_schema/cases is deliberately absent: its case file is
# malformed on purpose, so `serify schema` (rightly) refuses to load it.
CASE_DIRS=(
	examples/cases
	test/cases/audit/cases
	test/cases/happy/cases
	test/cases/wrong/cases
	test/cases/wrong/cases_crash
	test/cases/wrong/cases_error
	test/cases/wrong/cases_hang
)

check=false
case ${1:-} in
"") ;;
--check) check=true ;;
*)
	echo "usage: $0 [--check]" >&2
	exit 2
	;;
esac

go run ./cmd/serify schema "${CASE_DIRS[@]}"

if [[ $check == true ]]; then
	if ! git diff --quiet -- "${CASE_DIRS[@]}" || [[ -n $(git ls-files --others --exclude-standard -- "${CASE_DIRS[@]}") ]]; then
		echo >&2
		echo "error: generated schemas are out of date. Run scripts/gen-schemas.sh and commit the result." >&2
		git --no-pager diff --stat -- "${CASE_DIRS[@]}" >&2
		git ls-files --others --exclude-standard -- "${CASE_DIRS[@]}" >&2
		exit 1
	fi
	echo "schemas are up to date"
fi
