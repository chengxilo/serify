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

package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

// version is stamped by the release build with
// -ldflags "-X main.version=<tag>". A hardcoded literal here is the one copy
// of the version number nothing else updates, so it goes stale by default;
// resolveVersion recovers the real one instead.
var version = "dev"

// resolveVersion prefers the stamped version, then the module version the Go
// toolchain records for a `go install`ed binary, which covers the install path
// the README documents.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	root := NewRootCmd(resolveVersion())
	root.SetContext(ctx)
	err := root.Execute()

	cancel()
	if err != nil {
		os.Exit(1)
	}
}
