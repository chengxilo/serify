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

defmodule WorkerLibTest do
  use ExUnit.Case, async: true

  alias WorkerLib.Type

  # A model with a byte layout of its own: one u32, little-endian.
  defmodule Rec do
    use WorkerLib.Serify.Model

    defstruct [:n]

    serify_field(:n, :u32)

    def marshal(%__MODULE__{n: n}), do: <<n::little-32>>
    def unmarshal(<<n::little-32>>), do: %__MODULE__{n: n}
  end

  defp as_type do
    %{"rec" => %Type{model: Rec, formats: %{"binary" => {&Rec.marshal/1, &Rec.unmarshal/1}}}}
  end

  defp as_map do
    %{"rec" => %{"binary" => {fn _fm -> <<>> end, fn _data -> %{} end}}}
  end

  # ── resolve_registered ──────────────────────────────────────────────────
  #
  # The test a conformance run cannot replace. An unresolved (type, format) is
  # reported SKIPPED, so a spelling resolve_registered fails to understand
  # yields a *green* run made entirely of SKIPs, indistinguishable from a
  # worker that honestly does not implement the type.

  test "resolve_registered handles both registration shapes" do
    for {name, suite} <- [{"Type", as_type()}, {"map", as_map()}] do
      assert {ser, deser} = WorkerLib.resolve_registered(suite, "rec", "binary"),
             "#{name} registration resolved to nothing"

      assert is_function(ser, 1), "#{name} registration lost its serializer"
      assert is_function(deser, 1), "#{name} registration lost its deserializer"
    end
  end

  # The specific way this goes wrong in Elixir and nowhere else: %Type{} passes
  # is_map/1, so a resolver whose is_map clause is tried first reaches
  # Map.get(%Type{}, "binary"), gets nil, and reports SKIPPED. This asserts the
  # struct clause wins.
  test "a %Type{} is not mistaken for a format map, which is_map cannot tell apart" do
    assert is_map(%Type{model: Rec, formats: %{}}),
           "premise of this test: a struct is a map"

    assert Map.get(%Type{model: Rec, formats: %{"binary" => {&Rec.marshal/1, &Rec.unmarshal/1}}},
             "binary"
           ) == nil,
           "premise of this test: reading a format off the struct answers nil"

    assert {_, _} = WorkerLib.resolve_registered(as_type(), "rec", "binary")
  end

  test "unknown type or format resolves to nil" do
    assert WorkerLib.resolve_registered(as_type(), "rec", "json") == nil
    assert WorkerLib.resolve_registered(as_type(), "nope", "binary") == nil
    assert WorkerLib.resolve_registered(as_map(), "nope", "binary") == nil
  end

  # ── the model conversion itself ─────────────────────────────────────────

  test "a model-backed format converts field map <-> model on both sides" do
    {ser, deser} = WorkerLib.resolve_registered(as_type(), "rec", "binary")

    assert ser.(%{"n" => 7}) == <<7, 0, 0, 0>>
    assert deser.(<<9, 0, 0, 0>>) == %{"n" => 9}
  end

  test "a %Type{} with no model hands the functions the field map itself" do
    suite = %{
      "raw" => %Type{
        model: nil,
        formats: %{"binary" => {fn fm -> Map.fetch!(fm, "s") end, fn d -> %{"s" => d} end}}
      }
    }

    {ser, deser} = WorkerLib.resolve_registered(suite, "raw", "binary")
    assert ser.(%{"s" => "through"}) == "through"
    assert deser.("back") == %{"s" => "back"}
  end

  # ── the "*" wildcard, which backs the single-type WorkerLib.run/2 ────────

  test "the * wildcard still matches any (type, format)" do
    suite = %{"*" => %{"*" => {fn _ -> "w" end, fn _ -> %{} end}}}
    assert {_, _} = WorkerLib.resolve_registered(suite, "anything", "anyhow")
  end
end
