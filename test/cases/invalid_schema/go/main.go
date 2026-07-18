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
	"github.com/chengxilo/serify/lib/go/serify"
)

type Dumb struct {
	id string
}

func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"invalid_schema": {
				Model: &Dumb{},
				Formats: map[string]serify.Format{
					"byte": {
						Serializer: func(dumb Dumb) []byte {
							return []byte(dumb.id)
						},
						Deserializer: func(b []byte) Dumb {
							return Dumb{
								id: string(b),
							}
						},
					},
				},
			},
		},
	})
}
