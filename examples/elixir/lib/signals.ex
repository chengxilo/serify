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

defmodule SignalCapture do
  @moduledoc """
  Mirrors examples/cases/signals.yaml, which uses every scalar the schema allows
  as a list element.

  Elixir needs no per-width handling: integers are arbitrary-precision and the
  bitstring syntax takes the size and signedness directly, so each list is a
  comprehension over one literal pattern.

  Go is the --ref language and owns the byte layout; see examples/go/wire.go.
  Each list is a u32 element count followed by its elements, little-endian.
  """
  use WorkerLib.Serify.Model

  # Declaration order of the `mode` enum; the index is the wire ordinal.
  @signal_modes ["idle", "active", "fault", "calibrating"]

  defstruct [
    :capture_id, :flags, :raw_frame, :port_numbers, :sample_counts,
    :byte_totals, :trim_offsets, :drift_deltas, :temperatures_c,
    :timestamps_ns, :counters, :balances, :gains, :voltages,
    :channel_names, :payloads, :checksum, :window, :dropped_frames, :mode
  ]

  serify_field(:capture_id, :u64)
  serify_field(:flags, {:list, :bool})
  serify_field(:raw_frame, {:list, :u8})
  serify_field(:port_numbers, {:list, :u16})
  serify_field(:sample_counts, {:list, :u32})
  serify_field(:byte_totals, {:list, :u64})
  serify_field(:trim_offsets, {:list, :i8})
  serify_field(:drift_deltas, {:list, :i16})
  serify_field(:temperatures_c, {:list, :i32})
  serify_field(:timestamps_ns, {:list, :i64})
  serify_field(:counters, {:list, :u128})
  serify_field(:balances, {:list, :i128})
  serify_field(:gains, {:list, :f32})
  serify_field(:voltages, {:list, :f64})
  serify_field(:channel_names, {:list, :string})
  serify_field(:payloads, {:list, :bytes})
  serify_field(:checksum, {:array, :u8})
  serify_field(:window, {:array, :i16})
  serify_field(:dropped_frames, {:optional, :u32})
  serify_field(:mode, :string)

  def marshal(%__MODULE__{} = s) do
    IO.iodata_to_binary([
      <<s.capture_id::little-64>>,
      count_and(s.flags, fn v -> <<if(v, do: 1, else: 0)>> end),
      count_and(s.raw_frame, fn v -> <<v::little-8>> end),
      count_and(s.port_numbers, fn v -> <<v::little-16>> end),
      count_and(s.sample_counts, fn v -> <<v::little-32>> end),
      count_and(s.byte_totals, fn v -> <<v::little-64>> end),
      count_and(s.trim_offsets, fn v -> <<v::signed-little-8>> end),
      count_and(s.drift_deltas, fn v -> <<v::signed-little-16>> end),
      count_and(s.temperatures_c, fn v -> <<v::signed-little-32>> end),
      count_and(s.timestamps_ns, fn v -> <<v::signed-little-64>> end),
      count_and(s.counters, fn v -> <<v::little-128>> end),
      count_and(s.balances, fn v -> <<v::signed-little-128>> end),
      count_and(s.gains, fn v -> <<v::little-float-32>> end),
      count_and(s.voltages, fn v -> <<v::little-float-64>> end),
      count_and(s.channel_names, fn v -> <<byte_size(v)::little-32, v::binary>> end),
      count_and(s.payloads, fn v -> <<byte_size(v)::little-32, v::binary>> end),
      # array<T,N> carries no count: N is fixed by the schema.
      Enum.map(s.checksum || [], fn v -> <<v::little-8>> end),
      Enum.map(s.window || [], fn v -> <<v::signed-little-16>> end),
      # optional<uint32>: a presence flag, then the value if present.
      case s.dropped_frames do
        nil -> <<0>>
        v -> <<1, v::little-32>>
      end,
      # enum: a u8 ordinal, the variant's position in the case file.
      <<Enum.find_index(@signal_modes, &(&1 == s.mode))::little-8>>
    ])
  end

  # u32 element count, then each element rendered by `f`.
  defp count_and(list, f) do
    list = list || []
    [<<length(list)::little-32>> | Enum.map(list, f)]
  end

  def unmarshal(<<capture_id::little-64, rest::binary>>) do
    {flags, rest} = take(rest, 1, fn <<v::8>> -> v != 0 end)
    {raw_frame, rest} = take(rest, 1, fn <<v::little-8>> -> v end)
    {port_numbers, rest} = take(rest, 2, fn <<v::little-16>> -> v end)
    {sample_counts, rest} = take(rest, 4, fn <<v::little-32>> -> v end)
    {byte_totals, rest} = take(rest, 8, fn <<v::little-64>> -> v end)
    {trim_offsets, rest} = take(rest, 1, fn <<v::signed-little-8>> -> v end)
    {drift_deltas, rest} = take(rest, 2, fn <<v::signed-little-16>> -> v end)
    {temperatures_c, rest} = take(rest, 4, fn <<v::signed-little-32>> -> v end)
    {timestamps_ns, rest} = take(rest, 8, fn <<v::signed-little-64>> -> v end)
    {counters, rest} = take(rest, 16, fn <<v::little-128>> -> v end)
    {balances, rest} = take(rest, 16, fn <<v::signed-little-128>> -> v end)
    {gains, rest} = take(rest, 4, fn <<v::little-float-32>> -> v end)
    {voltages, rest} = take(rest, 8, fn <<v::little-float-64>> -> v end)
    {channel_names, rest} = take_var(rest, & &1)
    {payloads, rest} = take_var(rest, & &1)
    <<c0::little-8, c1::little-8, c2::little-8, c3::little-8,
      w0::signed-little-16, w1::signed-little-16, w2::signed-little-16, opt::binary>> = rest

    {dropped_frames, after_opt} =
      case opt do
        <<0, rest::binary>> -> {nil, rest}
        <<1, v::little-32, rest::binary>> -> {v, rest}
      end

    <<mode_ord::little-8, _::binary>> = after_opt
    mode = Enum.at(@signal_modes, mode_ord)

    %__MODULE__{
      capture_id: capture_id,
      flags: flags,
      raw_frame: raw_frame,
      port_numbers: port_numbers,
      sample_counts: sample_counts,
      byte_totals: byte_totals,
      trim_offsets: trim_offsets,
      drift_deltas: drift_deltas,
      temperatures_c: temperatures_c,
      timestamps_ns: timestamps_ns,
      counters: counters,
      balances: balances,
      gains: gains,
      voltages: voltages,
      channel_names: channel_names,
      payloads: payloads,
      checksum: [c0, c1, c2, c3],
      window: [w0, w1, w2],
      dropped_frames: dropped_frames,
      mode: mode
    }
  end

  # Read a u32 count, then that many fixed-width elements of `size` bytes.
  defp take(<<count::little-32, rest::binary>>, size, f) do
    total = count * size
    <<chunk::binary-size(total), tail::binary>> = rest
    items = for <<piece::binary-size(size) <- chunk>>, do: f.(piece)
    {items, tail}
  end

  # Read a u32 count, then that many length-prefixed byte strings.
  defp take_var(<<count::little-32, rest::binary>>, f) do
    Enum.reduce(1..count//1, {[], rest}, fn _, {acc, <<n::little-32, tail::binary>>} ->
      <<value::binary-size(n), tail::binary>> = tail
      {[f.(value) | acc], tail}
    end)
    |> then(fn {acc, tail} -> {Enum.reverse(acc), tail} end)
  end
end
