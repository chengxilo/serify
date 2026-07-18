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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chengxilo/serify/internal/report"
)

func newTableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "table <results.csv>",
		Short: "Render a results CSV (from `serify run --csv`) as a table",
		Long: `Render a results CSV as the same table that 'serify run' prints.

The CSV is the canonical result data; this command is just a readable view of it,
so you can re-render a saved run offline without re-running the workers.

Examples:
  serify table out.csv`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open %s: %w", args[0], err)
			}
			defer func() { _ = f.Close() }()

			recs, err := report.ReadCSV(f)
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			return report.RenderTable(os.Stdout, recs)
		},
	}
}
