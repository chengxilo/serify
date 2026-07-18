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
	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:          "serify",
		Short:        "Cross-language serialization conformance test runner",
		Version:      version,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newTableCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newSchemaCmd())

	return root
}
