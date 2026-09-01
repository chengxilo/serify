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

defmodule Request do
  @moduledoc """
  One request frame: cases/request.yaml.

  Elixir has no sum type to declare, and needs none — a tagged tuple already
  *is* a tag and a payload, so `serify_field(:op, :sum)` is the whole binding.
  No arm list, no converter, no registration. This is the cheapest of the nine
  bindings; Go, at the other end, writes out all five arms by hand.

      :list_all                a unit variant
      {:read, 1001}            a scalar payload
      {:create, %{...}}        a map payload is the struct holding N fields

  The price of declaring nothing is that last line: a struct payload arrives as
  a plain field map with string keys, because nothing told the binding which
  module it belongs to. `Draft.from_field_map/1` and `TaskItem.to_field_map/1` are
  the conversion, and they are the model's own generated functions — so the
  codec below still speaks structs throughout.
  """
  use WorkerLib.Serify.Model

  defstruct request_id: 0, op: :list_all

  serify_field(:request_id, :u32)
  serify_field(:op, :sum)

  def marshal(%__MODULE__{} = req) do
    # The tag ordinal is the arm's position in the case file's sum. The schema
    # tag *names* are the binding's business, and never appear here.
    tagged =
      case req.op do
        :list_all -> <<0>>
        {:create, m} -> <<1>> <> Draft.pack(Draft.from_field_map(m))
        {:read, id} -> <<2, id::little-64>>
        {:update, m} -> <<3>> <> TaskItem.pack(TaskItem.from_field_map(m))
        {:delete, id} -> <<4, id::little-64>>
        other -> raise ArgumentError, "unhandled op #{inspect(other)}"
      end

    <<req.request_id::little-32>> <> tagged
  end

  def unmarshal(<<request_id::little-32, tag::8, rest::binary>>) do
    op =
      case tag do
        0 ->
          :list_all

        1 ->
          {draft, _} = Draft.take(rest)
          {:create, Draft.to_field_map(draft)}

        2 ->
          <<id::little-64, _::binary>> = rest
          {:read, id}

        3 ->
          {task, _} = TaskItem.take(rest)
          {:update, TaskItem.to_field_map(task)}

        4 ->
          <<id::little-64, _::binary>> = rest
          {:delete, id}

        other ->
          raise ArgumentError, "unknown op tag #{other}"
      end

    %__MODULE__{request_id: request_id, op: op}
  end
end

defmodule Response do
  @moduledoc """
  One response frame: cases/response.yaml. `request_id` echoes the request's, so
  a client with several in flight can tell them apart.

  The arms are named for the outcome rather than the operation, because several
  operations share one: create, read and update all answer `:found`.
  """
  use WorkerLib.Serify.Model

  defstruct request_id: 0, result: :accepted

  serify_field(:request_id, :u32)
  serify_field(:result, :sum)

  def marshal(%__MODULE__{} = resp) do
    tagged =
      case resp.result do
        :accepted -> <<0>>
        {:found, m} -> <<1>> <> TaskItem.pack(TaskItem.from_field_map(m))
        {:listing, m} -> <<2>> <> TaskPage.pack(TaskPage.from_field_map(m))
        {:failed, m} -> <<3>> <> ApiError.pack(ApiError.from_field_map(m))
        other -> raise ArgumentError, "unhandled result #{inspect(other)}"
      end

    <<resp.request_id::little-32>> <> tagged
  end

  def unmarshal(<<request_id::little-32, tag::8, rest::binary>>) do
    result =
      case tag do
        0 ->
          :accepted

        1 ->
          {task, _} = TaskItem.take(rest)
          {:found, TaskItem.to_field_map(task)}

        2 ->
          {page, _} = TaskPage.take(rest)
          {:listing, TaskPage.to_field_map(page)}

        3 ->
          {err, _} = ApiError.take(rest)
          {:failed, ApiError.to_field_map(err)}

        other ->
          raise ArgumentError, "unknown result tag #{other}"
      end

    %__MODULE__{request_id: request_id, result: result}
  end
end
