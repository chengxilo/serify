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
	"reflect"

	"github.com/chengxilo/serify/lib/go/serify"
)

// This file is the whole worker: it names the types it can handle and hands
// serify one serializer/deserializer pair per format. Everything else lives in
// the files beside it — those stand in for the types an application already
// owns, each carrying its own schema binding and byte layout.
//
// This is the --ref worker, so it registers every type in the suite. A type a
// worker does not register is reported as SKIPPED rather than failing the run;
// see examples/cases/expected_skips/ for what the other workers still owe.
func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"customer": {
				Model: &CustomerRecord{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*CustomerRecord).MarshalBinary,
						Deserializer: (*CustomerRecord).UnmarshalBinary,
					},
					"json": {
						Serializer:   marshalJSON,
						Deserializer: unmarshalJSON,
					},
				},
			},
			"ledger": {
				Model: &LedgerEntry{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*LedgerEntry).MarshalBinary,
						Deserializer: (*LedgerEntry).UnmarshalBinary,
					},
				},
			},
			"order": {
				Model: &OrderRecord{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*OrderRecord).MarshalBinary,
						Deserializer: (*OrderRecord).UnmarshalBinary,
					},
				},
			},
			"telemetry": {
				Model: &TelemetryFrame{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*TelemetryFrame).MarshalBinary,
						Deserializer: (*TelemetryFrame).UnmarshalBinary,
					},
				},
			},
			"signals": {
				Model: &SignalCapture{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*SignalCapture).MarshalBinary,
						Deserializer: (*SignalCapture).UnmarshalBinary,
					},
				},
			},
			"notification": {
				Model: &NotificationRecord{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*NotificationRecord).MarshalBinary,
						Deserializer: (*NotificationRecord).UnmarshalBinary,
					},
				},
			},
		},
		Converters: map[reflect.Type]serify.Converter{
			reflect.TypeFor[Channel](): channelConverter,
		},
	})
}

var channelConverter = serify.NewConverter(
	func(v *serify.Variant) (Channel, error) {
		switch v.Tag {
		case "silent":
			return Silent{}, nil
		case "sms":
			s, ok := v.Value.(string)
			if !ok {
				return nil, fmt.Errorf("sms: expected string, got %T", v.Value)
			}
			return SMS(s), nil
		case "push":
			n, ok := v.Value.(uint64)
			if !ok {
				return nil, fmt.Errorf("push: expected uint64, got %T", v.Value)
			}
			return Push(n), nil
		case "invoice":
			fm, ok := v.Value.(*serify.FieldMap)
			if !ok {
				return nil, fmt.Errorf("invoice: expected a struct payload, got %T", v.Value)
			}
			currency, err := fm.GetString("currency")
			if err != nil {
				return nil, err
			}
			amount, err := fm.GetI64("amount_minor")
			if err != nil {
				return nil, err
			}
			return Invoice{Currency: currency, AmountMinor: amount}, nil
		default:
			return nil, fmt.Errorf("unknown channel variant %q", v.Tag)
		}
	},
	func(c Channel) *serify.Variant {
		switch ch := c.(type) {
		case Silent:
			return &serify.Variant{Tag: "silent"}
		case SMS:
			return &serify.Variant{Tag: "sms", Value: string(ch)}
		case Push:
			return &serify.Variant{Tag: "push", Value: uint64(ch)}
		case Invoice:
			fm := serify.NewFieldMap()
			fm.SetString("currency", ch.Currency)
			fm.SetI64("amount_minor", ch.AmountMinor)
			return &serify.Variant{Tag: "invoice", Value: fm}
		default:
			panic(fmt.Sprintf("notification: unhandled channel %T", c))
		}
	},
)
