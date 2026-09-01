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

defmodule Worker do
  @moduledoc """
  The conformance worker: serify's entire footprint in the Elixir follower.

  It registers the two types that cross the socket and hands each the very
  functions Client calls — `Request.marshal/1` is not a test double.
  """

  def main(_args) do
    WorkerLib.run_suite(%{
      "request" => %WorkerLib.Type{
        model: Request,
        formats: %{"binary" => {&Request.marshal/1, &Request.unmarshal/1}}
      },
      "response" => %WorkerLib.Type{
        model: Response,
        formats: %{"binary" => {&Response.marshal/1, &Response.unmarshal/1}}
      }
    })
  end
end
