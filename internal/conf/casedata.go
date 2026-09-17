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

package conf

import (
	"fmt"
	"math/big"

	"gopkg.in/yaml.v3"

	"github.com/chengxilo/serify/internal/kind"
)

// Case data is decoded schema-directed, not type-guessed: letting yaml.v3 pick
// Go types degrades an integer literal that overflows uint64 to float64. The
// raw yaml.Nodes are kept instead and each field is decoded into the type its
// schema entry demands, 64/128-bit integers through big.Int with a range check.

// rawTestCase mirrors TestCase but keeps the data values as yaml.Nodes so they
// can be decoded schema-directed after the schema is resolved.
type rawTestCase struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Data        map[string]yaml.Node `yaml:"data"`
}

// 128-bit integer bounds.
var (
	maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	maxInt128  = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	minInt128  = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
)

// decodeCaseData converts one case's raw data nodes into Go values using the
// schema. Fields not in the schema are decoded generically so validate() can
// still report them with its usual "not in schema" error.
func decodeCaseData(schema []Field, raw map[string]yaml.Node) (map[string]any, error) {
	byName := make(map[string]FieldType, len(schema))
	for _, f := range schema {
		byName[f.Name] = f.Type
	}
	out := make(map[string]any, len(raw))
	for k := range raw {
		node := raw[k]
		ft, known := byName[k]
		if !known {
			var v any
			if err := node.Decode(&v); err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			out[k] = v
			continue
		}
		v, err := decodeDataValue(ft, &node)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		out[k] = v
	}
	return out, nil
}

func decodeDataValue(ft FieldType, n *yaml.Node) (any, error) {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	if n.Tag == "!!null" {
		return nil, nil
	}

	switch ft.Base {
	case kind.Uint64:
		b, err := decodeExactInt(n)
		if err != nil {
			return nil, err
		}
		if !b.IsUint64() {
			return nil, fmt.Errorf("value %s out of range for uint64", b)
		}
		return b.Uint64(), nil

	case kind.Int64:
		b, err := decodeExactInt(n)
		if err != nil {
			return nil, err
		}
		if !b.IsInt64() {
			return nil, fmt.Errorf("value %s out of range for int64", b)
		}
		return b.Int64(), nil

	case kind.Uint128:
		b, err := decodeExactInt(n)
		if err != nil {
			return nil, err
		}
		if b.Sign() < 0 || b.Cmp(maxUint128) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint128", b)
		}
		return b, nil

	case kind.Int128:
		b, err := decodeExactInt(n)
		if err != nil {
			return nil, err
		}
		if b.Cmp(minInt128) < 0 || b.Cmp(maxInt128) > 0 {
			return nil, fmt.Errorf("value %s out of range for int128", b)
		}
		return b, nil

	case kind.Optional:
		if ft.Elem == nil {
			break
		}
		return decodeDataValue(*ft.Elem, n) // null was handled above

	case kind.List, kind.Array:
		if ft.Elem == nil {
			break
		}
		var items []yaml.Node
		if err := n.Decode(&items); err != nil {
			return nil, err
		}
		out := make([]any, len(items))
		for i := range items {
			v, err := decodeDataValue(*ft.Elem, &items[i])
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = v
		}
		return out, nil

	case kind.Map:
		if ft.Elem == nil {
			break
		}
		var m map[string]yaml.Node
		if err := n.Decode(&m); err != nil {
			return nil, err
		}
		out := make(map[string]any, len(m))
		for k := range m {
			item := m[k]
			v, err := decodeDataValue(*ft.Elem, &item)
			if err != nil {
				return nil, fmt.Errorf("[%q]: %w", k, err)
			}
			out[k] = v
		}
		return out, nil

	case kind.Struct:
		var m map[string]yaml.Node
		if err := n.Decode(&m); err != nil {
			return nil, err
		}
		return decodeCaseData(ft.Fields, m)

	case kind.Sum:
		// A variant is either a bare tag (unit variant) or a single-key map
		// {tag: payload}. Canonical in-memory form is always {tag: payload}
		// (payload nil for unit), so it travels uniformly.
		if n.Kind == yaml.ScalarNode {
			var tag string
			if err := n.Decode(&tag); err != nil {
				return nil, err
			}
			v := findVariant(ft.Variants, tag)
			if v == nil {
				return nil, fmt.Errorf("unknown variant %q", tag)
			}
			if v.Type != nil {
				return nil, fmt.Errorf("variant %q needs a payload {%s: ...}", tag, tag)
			}
			return map[string]any{tag: nil}, nil
		}
		var m map[string]yaml.Node
		if err := n.Decode(&m); err != nil {
			return nil, err
		}
		if len(m) != 1 {
			return nil, fmt.Errorf("sum value must name exactly one variant, got %d keys", len(m))
		}
		for tag := range m {
			v := findVariant(ft.Variants, tag)
			if v == nil {
				return nil, fmt.Errorf("unknown variant %q", tag)
			}
			if v.Type == nil {
				return map[string]any{tag: nil}, nil // unit written as {tag: null}
			}
			node := m[tag]
			payload, err := decodeDataValue(*v.Type, &node)
			if err != nil {
				return nil, fmt.Errorf("variant %q: %w", tag, err)
			}
			return map[string]any{tag: payload}, nil
		}
	}

	// All other kinds (small ints, floats incl. .nan/.inf, bool, string,
	// bytes in either form, enum) keep the generic decoding.
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// findVariant returns the variant with the given tag, or nil.
func findVariant(vs []Variant, tag string) *Variant {
	for i := range vs {
		if vs[i].Name == tag {
			return &vs[i]
		}
	}
	return nil
}

// decodeExactInt parses an integer scalar with full precision via big.Int's
// text unmarshalling. It accepts a bare integer literal of any size or a
// quoted decimal string, and rejects float forms (1e3, 1.5) — an integer
// field must be written as an integer.
func decodeExactInt(n *yaml.Node) (*big.Int, error) {
	var b big.Int
	if err := n.Decode(&b); err != nil {
		return nil, fmt.Errorf("expected an integer (bare or quoted decimal), got %q", n.Value)
	}
	return &b, nil
}
