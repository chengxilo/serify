// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chengxilo/serify/internal/language"
)

type marker struct {
	pattern  string
	language string
	isGlob   bool
}

var markers = []marker{
	{"go.mod", language.Go, false},
	{"Cargo.toml", language.Rust, false},
	{"pom.xml", language.Java, false},
	{"mix.exs", language.Elixir, false},
	{"*.csproj", language.CSharp, true},
	{"package.json", language.Node, false},
	{"composer.json", language.PHP, false},
	{"*.py", language.Python, true},
	{"*.cpp", language.Cpp, true},
	{"*.php", language.PHP, true},
	{"*.go", language.Go, true},
}

// DetectLanguage identifies the programming language of a worker directory
// by checking for well-known marker files.
func DetectLanguage(dir string) (string, error) {
	var found []string
	seen := make(map[string]bool)

	for _, m := range markers {
		var exists bool
		if m.isGlob {
			matches, err := filepath.Glob(filepath.Join(dir, m.pattern))
			if err != nil {
				return "", fmt.Errorf("glob %s in %s: %w", m.pattern, dir, err)
			}
			exists = len(matches) > 0
		} else {
			_, err := os.Stat(filepath.Join(dir, m.pattern))
			exists = err == nil
		}

		if exists && !seen[m.language] {
			seen[m.language] = true
			found = append(found, m.language)
		}
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("cannot detect language in %s: no recognized marker files found", dir)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf(
			"ambiguous language in %s: multiple languages detected (%s); make sure each worker has its own directory",
			dir, strings.Join(found, ", "))
	}
}
