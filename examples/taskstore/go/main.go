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

	"github.com/chengxilo/serify/example-taskstore/api"
)

// The conformance worker: the whole of serify's presence in this project.
//
// It registers the two types that cross the socket and hands each one the very
// serializer the server uses — api.Request.MarshalBinary is not a test double,
// it is the function cmd/server calls on every reply. That is the point of the
// arrangement: if this worker passes, the server's own bytes are what the
// followers were checked against.
//
// Everything below the Run call is the price Go charges for not having a sum
// type. See the comment on opConverter.
func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"request": {
				Model: &api.Request{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*api.Request).MarshalBinary,
						Deserializer: (*api.Request).UnmarshalBinary,
					},
				},
			},
			"response": {
				Model: &api.Response{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*api.Response).MarshalBinary,
						Deserializer: (*api.Response).UnmarshalBinary,
					},
				},
			},
		},
		Converters: map[reflect.Type]serify.Converter{
			reflect.TypeFor[api.Op]():     opConverter,
			reflect.TypeFor[api.Result](): resultConverter,
		},
	})
}

// opConverter teaches serify how api.Op maps to a schema sum, in both
// directions.
//
// Go is one of the three languages that needs this. A sum binds onto whatever
// sum type the language already has, and six of the nine can be read by the
// binding on their own — a Rust enum, a Python union of dataclasses, a Java
// sealed interface. Go's sealed interface cannot: there is no way to enumerate
// the implementations of an interface at run time, so the arms have to be named
// somewhere, and this is that somewhere.
//
// Compare python/worker.py, which registers the same two types and declares no
// converter at all.
var opConverter = serify.NewConverter(
	func(v *serify.Variant) (api.Op, error) {
		switch v.Tag {
		case "list_all":
			return api.ListAll{}, nil
		case "create":
			fm, err := payload(v)
			if err != nil {
				return nil, err
			}
			d, err := draftFromFieldMap(fm)
			return api.Create(d), err
		case "read":
			id, ok := v.Value.(uint64)
			if !ok {
				return nil, fmt.Errorf("read: expected uint64, got %T", v.Value)
			}
			return api.Read(id), nil
		case "update":
			fm, err := payload(v)
			if err != nil {
				return nil, err
			}
			t, err := taskFromFieldMap(fm)
			return api.Update(t), err
		case "delete":
			id, ok := v.Value.(uint64)
			if !ok {
				return nil, fmt.Errorf("delete: expected uint64, got %T", v.Value)
			}
			return api.Delete(id), nil
		default:
			return nil, fmt.Errorf("unknown op %q", v.Tag)
		}
	},
	func(op api.Op) *serify.Variant {
		switch v := op.(type) {
		case api.ListAll:
			return &serify.Variant{Tag: "list_all"} // a unit variant carries no payload
		case api.Create:
			return &serify.Variant{Tag: "create", Value: draftToFieldMap(api.Draft(v))}
		case api.Read:
			return &serify.Variant{Tag: "read", Value: uint64(v)}
		case api.Update:
			return &serify.Variant{Tag: "update", Value: taskToFieldMap(api.Task(v))}
		case api.Delete:
			return &serify.Variant{Tag: "delete", Value: uint64(v)}
		default:
			panic(fmt.Sprintf("taskstore: unhandled op %T", op))
		}
	},
)

var resultConverter = serify.NewConverter(
	func(v *serify.Variant) (api.Result, error) {
		switch v.Tag {
		case "accepted":
			return api.Accepted{}, nil
		case "found":
			fm, err := payload(v)
			if err != nil {
				return nil, err
			}
			t, err := taskFromFieldMap(fm)
			return api.Found(t), err
		case "listing":
			fm, err := payload(v)
			if err != nil {
				return nil, err
			}
			p, err := pageFromFieldMap(fm)
			return api.Listing(p), err
		case "failed":
			fm, err := payload(v)
			if err != nil {
				return nil, err
			}
			e, err := apiErrorFromFieldMap(fm)
			return api.Failed(e), err
		default:
			return nil, fmt.Errorf("unknown result %q", v.Tag)
		}
	},
	func(r api.Result) *serify.Variant {
		switch v := r.(type) {
		case api.Accepted:
			return &serify.Variant{Tag: "accepted"}
		case api.Found:
			return &serify.Variant{Tag: "found", Value: taskToFieldMap(api.Task(v))}
		case api.Listing:
			return &serify.Variant{Tag: "listing", Value: pageToFieldMap(api.TaskPage(v))}
		case api.Failed:
			return &serify.Variant{Tag: "failed", Value: apiErrorToFieldMap(api.APIError(v))}
		default:
			panic(fmt.Sprintf("taskstore: unhandled result %T", r))
		}
	},
)

// payload asserts that a variant carries a struct, which is how an arity-N arm
// travels.
func payload(v *serify.Variant) (*serify.FieldMap, error) {
	fm, ok := v.Value.(*serify.FieldMap)
	if !ok {
		return nil, fmt.Errorf("%s: expected a struct payload, got %T", v.Tag, v.Value)
	}
	return fm, nil
}

// The struct payloads, field by field. This is the one place the schema's field
// names are written out by hand; everywhere else in the project they come from
// the `serify:"…"` tags on the structs themselves.

func taskToFieldMap(t api.Task) *serify.FieldMap {
	fm := serify.NewFieldMap()
	fm.SetU64("id", t.ID)
	fm.SetString("title", t.Title)
	fm.SetBool("done", t.Done)
	fm.SetString("priority", t.Priority)
	fm.SetListString("tags", t.Tags)
	serify.SetOptional(fm, "due_at", t.DueAt)
	return fm
}

func taskFromFieldMap(fm *serify.FieldMap) (api.Task, error) {
	var t api.Task
	var err error
	if t.ID, err = fm.GetU64("id"); err != nil {
		return t, err
	}
	if t.Title, err = fm.GetString("title"); err != nil {
		return t, err
	}
	if t.Done, err = fm.GetBool("done"); err != nil {
		return t, err
	}
	if t.Priority, err = fm.GetString("priority"); err != nil {
		return t, err
	}
	if t.Tags, err = fm.GetListString("tags"); err != nil {
		return t, err
	}
	t.DueAt, err = serify.GetOptional[int64](fm, "due_at")
	return t, err
}

func draftToFieldMap(d api.Draft) *serify.FieldMap {
	fm := serify.NewFieldMap()
	fm.SetString("title", d.Title)
	fm.SetString("priority", d.Priority)
	fm.SetListString("tags", d.Tags)
	serify.SetOptional(fm, "due_at", d.DueAt)
	return fm
}

func draftFromFieldMap(fm *serify.FieldMap) (api.Draft, error) {
	var d api.Draft
	var err error
	if d.Title, err = fm.GetString("title"); err != nil {
		return d, err
	}
	if d.Priority, err = fm.GetString("priority"); err != nil {
		return d, err
	}
	if d.Tags, err = fm.GetListString("tags"); err != nil {
		return d, err
	}
	d.DueAt, err = serify.GetOptional[int64](fm, "due_at")
	return d, err
}

func pageToFieldMap(p api.TaskPage) *serify.FieldMap {
	items := make([]*serify.FieldMap, len(p.Items))
	for i, t := range p.Items {
		items[i] = taskToFieldMap(t)
	}
	fm := serify.NewFieldMap()
	fm.SetListStruct("items", items)
	fm.SetU32("total", p.Total)
	return fm
}

func pageFromFieldMap(fm *serify.FieldMap) (api.TaskPage, error) {
	var p api.TaskPage
	items, err := fm.GetListStruct("items")
	if err != nil {
		return p, err
	}
	p.Items = make([]api.Task, len(items))
	for i, item := range items {
		if p.Items[i], err = taskFromFieldMap(item); err != nil {
			return p, err
		}
	}
	p.Total, err = fm.GetU32("total")
	return p, err
}

func apiErrorToFieldMap(e api.APIError) *serify.FieldMap {
	fm := serify.NewFieldMap()
	fm.SetString("code", e.Code)
	fm.SetString("message", e.Message)
	return fm
}

func apiErrorFromFieldMap(fm *serify.FieldMap) (api.APIError, error) {
	var e api.APIError
	var err error
	if e.Code, err = fm.GetString("code"); err != nil {
		return e, err
	}
	e.Message, err = fm.GetString("message")
	return e, err
}
