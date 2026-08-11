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

import "github.com/chengxilo/serify/internal/language"

// LangDefault holds the default build and run commands for a language.
type LangDefault struct {
	Build string
	Run   string
}

// Defaults maps each supported language to its default build/run commands, used
// when a worker directory has no worker.yaml overriding them. They assume a
// self-contained worker laid out the conventional way for its language. Every
// worker in this repo does override them, because each has to reach a library
// under lib/ by relative path — that is a property of living in the repo, not a
// sign the defaults are wrong.
var Defaults = map[string]LangDefault{
	language.Go: {
		Build: "go build -o worker .",
		Run:   "./worker",
	},
	language.Rust: {
		Build: "cargo build --release",
		Run:   "./target/release/worker",
	},
	language.Python: {
		Build: "",
		Run:   "python worker.py",
	},
	language.Node: {
		Build: "npm install && npx tsc",
		Run:   "node dist/worker.js",
	},
	language.Java: {
		Build: "mvn -q package -DskipTests",
		Run:   "java -jar target/worker.jar",
	},
	language.Cpp: {
		Build: "g++ -O2 -std=c++17 -o worker worker.cpp",
		Run:   "./worker",
	},
	language.CSharp: {
		Build: "dotnet build -c Release -o bin",
		Run:   "dotnet bin/worker.dll",
	},
	language.Elixir: {
		Build: "mix deps.get && mix compile",
		Run:   "mix run lib/worker.ex",
	},
	language.PHP: {
		Build: "composer install --no-dev",
		Run:   "php worker.php",
	},
}
