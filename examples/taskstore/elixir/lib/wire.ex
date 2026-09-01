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

defmodule Wire do
  @moduledoc """
  Byte-level primitives. Go owns the layout these reproduce; the conventions
  are documented at the top of go/api/wire.go.

  Elixir has no cursor: binary pattern matching takes the head off and hands
  back the tail, so every reader here returns `{value, rest}`. Same layout as
  everywhere else — the byte order and the widths are the contract, not the
  shape of the code that walks them.
  """

  @priorities ["low", "normal", "high"]
  @error_codes ["not_found", "invalid", "conflict"]

  @doc "Declaration order of the `enum<low, normal, high>` in cases/task.yaml."
  def priorities, do: @priorities

  @doc "Declaration order of the enum in cases/api_error.yaml."
  def error_codes, do: @error_codes

  @doc "A u32 byte length followed by the bytes themselves."
  def str(s), do: <<byte_size(s)::little-32, s::binary>>

  def take_str(<<n::little-32, s::binary-size(n), rest::binary>>), do: {s, rest}

  def bool(true), do: <<1>>
  def bool(false), do: <<0>>

  @doc """
  An enum as the ordinal of its position in `variants`.

  The ordinal is this codec's own byte-layout choice — an enum travels through
  serify as its *name* — so the list has to match the case file's declaration
  order.
  """
  def enum(variants, name) do
    case Enum.find_index(variants, &(&1 == name)) do
      nil -> raise ArgumentError, "#{inspect(name)} is not one of #{inspect(variants)}"
      ord -> <<ord::8>>
    end
  end

  def take_enum(variants, <<ord::8, rest::binary>>) do
    case Enum.at(variants, ord) do
      nil -> raise ArgumentError, "enum ordinal #{ord} out of range"
      name -> {name, rest}
    end
  end

  def tags(tags), do: <<length(tags)::little-32>> <> Enum.map_join(tags, &str/1)

  def take_tags(<<n::little-32, rest::binary>>), do: take_n(rest, n, &take_str/1)

  def optional_i64(nil), do: <<0>>
  def optional_i64(v), do: <<1, v::signed-little-64>>

  def take_optional_i64(<<0, rest::binary>>), do: {nil, rest}
  def take_optional_i64(<<1, v::signed-little-64, rest::binary>>), do: {v, rest}

  @doc "Take `n` items with `fun`, returning them in order plus the remaining bytes."
  def take_n(data, n, fun) do
    Enum.reduce(1..n//1, {[], data}, fn _, {acc, rest} ->
      {item, rest} = fun.(rest)
      {[item | acc], rest}
    end)
    |> then(fn {acc, rest} -> {Enum.reverse(acc), rest} end)
  end
end
