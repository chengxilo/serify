# Copyright 2026 Chengxi Luo
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""The conformance worker: serify's entire footprint in the Python follower.

It registers the same two types the Go worker does and hands each the very
functions client.py calls — api.Request.marshal is not a test double.

What it does *not* contain is a converter. Go needs one for its two sums because
a sealed interface cannot be enumerated at run time; Python's union of
dataclasses names its own arms, so the binding reads them straight off the type.
Compare go/main.go, where the same five operations are spelled out twice.
"""

import os
import sys

_lib_dir = os.path.join(os.path.dirname(__file__), '../../../lib/python')
if not os.path.isdir(_lib_dir):
    raise RuntimeError(f"serify library not found at {_lib_dir}; fix the relative path")
sys.path.insert(0, _lib_dir)

import api  # noqa: E402
from serify import Format, Type, run_suite, serify_model  # noqa: E402

# Apply the schema binding here rather than as a decorator in api.py, so that
# module — the one the server and the client actually use — imports nothing but
# the standard library. `serify_model` mutates the class and hands it back, so
# calling it late is the same as decorating it early.
#
# The sum arms are deliberately absent: an arm is a plain dataclass and the
# binding reads it through the union, and a unit variant has no fields for
# serify_model to bind anyway.
for _model in (api.Task, api.Draft, api.TaskPage, api.ApiError, api.Request, api.Response):
    serify_model(_model)

if __name__ == '__main__':
    run_suite({
        "request": Type(api.Request, {
            "binary": Format(api.Request.marshal, api.Request.unmarshal),
        }),
        "response": Type(api.Response, {
            "binary": Format(api.Response.marshal, api.Response.unmarshal),
        }),
    })
