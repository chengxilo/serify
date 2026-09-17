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

import "github.com/chengxilo/serify/internal/lang"

// LangDefault holds the default build and run commands for a language.
type LangDefault struct {
	Build string
	Run   string
}

// Defaults maps each supported language to its default build/run commands, used
// when a worker directory has no worker.yaml overriding them. They assume a
// self-contained worker; every worker in this repo overrides them, because each
// reaches a library under lib/ by relative path.
var Defaults = map[string]LangDefault{
	lang.Go: {
		Build: "go build -o worker .",
		Run:   "./worker",
	},
	lang.Rust: {
		Build: "cargo build --release",
		Run:   "./target/release/worker",
	},
	lang.Python: {
		Build: "",
		Run:   "python worker.py",
	},
	lang.Node: {
		Build: "npm install && npx tsc",
		Run:   "node dist/worker.js",
	},
	lang.Java: {
		Build: "mvn -q package -DskipTests",
		Run:   "java -jar target/worker.jar",
	},
	lang.Cpp: {
		Build: "g++ -O2 -std=c++17 -o worker worker.cpp",
		Run:   "./worker",
	},
	lang.CSharp: {
		Build: "dotnet build -c Release -o bin",
		Run:   "dotnet bin/worker.dll",
	},
	lang.Elixir: {
		Build: "mix deps.get && mix compile",
		Run:   "mix run lib/worker.ex",
	},
	lang.PHP: {
		Build: "composer install --no-dev",
		Run:   "php worker.php",
	},
}
