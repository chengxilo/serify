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

# audit_model: the same faults over the same layout, through the model path.
# `mutating` stays silent here for a reason of its own: every BEAM term is
# immutable, so a serializer cannot mutate the struct it was handed. It is
# registered anyway so the grid has no holes.
defmodule AuditModel do
  @moduledoc false
  use WorkerLib.Serify.Model

  defstruct [:payload, :tag, :value, :tags]

  serify_field(:payload, :bytes)
  serify_field(:tag, :string)
  serify_field(:value, :u32)
  serify_field(:tags, {:list, :string})
end

defmodule AuditWorker do
  def main(_), do: WorkerLib.run_suite(%{
    "audit_model" => %WorkerLib.Type{
      model: AuditModel,
      formats: %{
        "clean" => {&model_ser/1, &model_deser/1},
        "mutating" => {&model_mut_ser/1, &model_deser/1},
        "unstable" => {&model_unstable_ser/1, &model_deser/1},
      }
    },
    "audit" => %{
      "clean" => {&clean_ser/1, &clean_deser/1},
      "mutating" => {&mut_ser/1, &clean_deser/1},
      "unstable" => {&unstable_ser/1, &clean_deser/1},
      "deser-unstable" => {&clean_ser/1, &du_deser/1},
      "input-mutating" => {&clean_ser/1, &im_deser/1},
    }
  })

  defp marshal(fm) do
    t = fm["tag"]; p = fm["payload"]; tags = fm["tags"]
    IO.iodata_to_binary([
      <<fm["value"]::little-32, byte_size(t), t::binary,
        byte_size(p)::little-32, p::binary, length(tags)>>,
      Enum.map(tags, fn s -> <<byte_size(s), s::binary>> end),
    ])
  end

  defp unmarshal(data, copy_payload) do
    <<val::little-32, tlen, rest::binary>> = data
    <<t::binary-size(tlen), rest::binary>> = rest
    <<plen::little-32, rest::binary>> = rest
    <<p::binary-size(plen), rest::binary>> = rest
    <<tc, rest::binary>> = rest
    {tags, _} = take_tags(rest, tc, [])
    p = if copy_payload, do: p, else: p
    %{"value" => val, "tag" => t, "payload" => p, "tags" => tags}
  end

  defp take_tags(rest, 0, acc), do: {Enum.reverse(acc), rest}
  defp take_tags(<<tl, rest::binary>>, n, acc) do
    <<s::binary-size(tl), rest::binary>> = rest
    take_tags(rest, n - 1, [s | acc])
  end

  # Counters via process dictionary (single-threaded worker, safe).
  defp next_ctr(key) do
    c = Process.get(key, 0)
    Process.put(key, c + 1)
    c
  end

  # --- audit_model codecs ---------------------------------------------------

  defp marshal_model(%AuditModel{} = m) do
    IO.iodata_to_binary([
      <<m.value::little-32, byte_size(m.tag), m.tag::binary,
        byte_size(m.payload)::little-32, m.payload::binary, length(m.tags)>>,
      Enum.map(m.tags, fn s -> <<byte_size(s), s::binary>> end),
    ])
  end

  defp model_ser(m), do: marshal_model(m)

  defp model_deser(data) do
    <<val::little-32, tlen, rest::binary>> = data
    <<t::binary-size(tlen), rest::binary>> = rest
    <<plen::little-32, rest::binary>> = rest
    <<p::binary-size(plen), rest::binary>> = rest
    <<tc, rest::binary>> = rest
    {tags, _} = take_tags(rest, tc, [])
    %AuditModel{value: val, tag: t, payload: p, tags: tags}
  end

  # "Mutation" on the BEAM rebinds a local name and nothing else: the caller's
  # struct is untouched.
  defp model_mut_ser(m) do
    data = marshal_model(m)
    _ = %{m | value: 0}
    data
  end

  # The positive control: this fault shows in the returned bytes, so it reports
  # whether or not the model survives the call.
  defp model_unstable_ser(m), do: marshal_model(m) <> <<next_ctr(:model_unstable)>>

  defp clean_ser(fm), do: marshal(fm)
  defp clean_deser(d), do: unmarshal(d, true)
  defp mut_ser(fm) do
    b = marshal(fm)
    Map.put(fm, "value", 0)
    b
  end

  defp unstable_ser(fm) do
    c = next_ctr(:uctr)
    marshal(fm) <> <<c>>
  end

  defp du_deser(d) do
    c = next_ctr(:ductr)
    fm = unmarshal(d, true)
    if c > 0, do: Map.put(fm, "value", fm["value"] + 1), else: fm
  end

  defp im_deser(d) do
    fm = unmarshal(d, true)
    # Mutate the input binary — in Erlang/Elixir binaries are immutable,
    # so this is a no-op here (the audit test will just see no mutation).
    fm
  end
end
