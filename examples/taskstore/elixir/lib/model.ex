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

defmodule TaskItem do
  @moduledoc """
  A stored task: cases/task.yaml.

  Named `TaskItem` rather than `Task` because Elixir already has a `Task`
  module, and shadowing it breaks mix itself — `Task.async/1` is what the
  compiler uses to consolidate protocols. The module name never reaches the
  wire: the schema binds field names, and the sum tags are the tuple atoms in
  message.ex, so renaming it costs nothing.

  `use WorkerLib.Serify.Model` plus one `serify_field` per field is the entire
  schema binding. An enum needs nothing from it — it travels as its variant
  *name*, so `:priority` is a plain string and `Wire.priorities/0` fixes the
  ordinal this codec writes.
  """
  use WorkerLib.Serify.Model

  defstruct id: 0, title: "", done: false, priority: "normal", tags: [], due_at: nil

  serify_field(:id, :u64)
  serify_field(:title, :string)
  serify_field(:done, :bool)
  serify_field(:priority, :string)
  serify_field(:tags, {:list, :string})
  serify_field(:due_at, {:optional, :i64})

  def pack(%__MODULE__{} = t) do
    <<t.id::little-64>> <>
      Wire.str(t.title) <>
      Wire.bool(t.done) <>
      Wire.enum(Wire.priorities(), t.priority) <>
      Wire.tags(t.tags) <>
      Wire.optional_i64(t.due_at)
  end

  def take(<<id::little-64, rest::binary>>) do
    {title, rest} = Wire.take_str(rest)
    <<done::8, rest::binary>> = rest
    {priority, rest} = Wire.take_enum(Wire.priorities(), rest)
    {tags, rest} = Wire.take_tags(rest)
    {due_at, rest} = Wire.take_optional_i64(rest)

    {%__MODULE__{
       id: id,
       title: title,
       done: done == 1,
       priority: priority,
       tags: tags,
       due_at: due_at
     }, rest}
  end
end

defmodule Draft do
  @moduledoc "A task the server has not assigned an id to yet: cases/draft.yaml."
  use WorkerLib.Serify.Model

  defstruct title: "", priority: "normal", tags: [], due_at: nil

  serify_field(:title, :string)
  serify_field(:priority, :string)
  serify_field(:tags, {:list, :string})
  serify_field(:due_at, {:optional, :i64})

  def pack(%__MODULE__{} = d) do
    Wire.str(d.title) <>
      Wire.enum(Wire.priorities(), d.priority) <>
      Wire.tags(d.tags) <>
      Wire.optional_i64(d.due_at)
  end

  def take(data) do
    {title, rest} = Wire.take_str(data)
    {priority, rest} = Wire.take_enum(Wire.priorities(), rest)
    {tags, rest} = Wire.take_tags(rest)
    {due_at, rest} = Wire.take_optional_i64(rest)

    {%__MODULE__{title: title, priority: priority, tags: tags, due_at: due_at}, rest}
  end
end

defmodule TaskPage do
  @moduledoc "The body of a list response: cases/task_page.yaml."
  use WorkerLib.Serify.Model

  defstruct items: [], total: 0

  serify_field(:items, {:list, :struct}, module: TaskItem)
  serify_field(:total, :u32)

  def pack(%__MODULE__{} = p) do
    <<length(p.items)::little-32>> <>
      Enum.map_join(p.items, &TaskItem.pack/1) <>
      <<p.total::little-32>>
  end

  def take(<<n::little-32, rest::binary>>) do
    {items, rest} = Wire.take_n(rest, n, &TaskItem.take/1)
    <<total::little-32, rest::binary>> = rest
    {%__MODULE__{items: items, total: total}, rest}
  end
end

defmodule ApiError do
  @moduledoc "A failed request: cases/api_error.yaml."
  use WorkerLib.Serify.Model

  defstruct code: "not_found", message: ""

  serify_field(:code, :string)
  serify_field(:message, :string)

  def pack(%__MODULE__{} = e) do
    Wire.enum(Wire.error_codes(), e.code) <> Wire.str(e.message)
  end

  def take(data) do
    {code, rest} = Wire.take_enum(Wire.error_codes(), data)
    {message, rest} = Wire.take_str(rest)
    {%__MODULE__{code: code, message: message}, rest}
  end
end
