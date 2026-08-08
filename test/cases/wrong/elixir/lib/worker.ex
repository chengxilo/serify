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

defmodule WrongWorker do
  @self_lang "elixir"

  def main(_args) do
    WorkerLib.run_suite(%{
      "wrong" => %{
        "binary" => {&binary_serialize/1, &binary_deserialize/1},
        "json" => {&json_serialize/1, &json_deserialize/1},
        "err_ser" => {&err_ser/1, &binary_deserialize/1},
        "err_deser" => {&binary_serialize/1, &err_deser/1},
        "hang" => {&hang_ser/1, &binary_deserialize/1},
        "crash" => {&crash_ser/1, &binary_deserialize/1},
      }
    })
  end

  defp to_upper_self(langs), do: Enum.map(langs, fn s -> if s == @self_lang, do: "ELIXIR", else: s end)

  # --- binary ----------------------------------------------------------------

  defp binary_serialize(fm) do
    langs = if fm["binary_serialize"], do: fm["langs"], else: to_upper_self(fm["langs"])
    flags = [
      if(fm["binary_serialize"], do: 1, else: 0),
      if(fm["binary_deserialize"], do: 1, else: 0),
      if(fm["json_serialize"], do: 1, else: 0),
      if(fm["json_deserialize"], do: 1, else: 0),
    ]
    IO.iodata_to_binary([
      flags,
      <<length(langs)::little-32>>,
      Enum.map(langs, fn s -> <<byte_size(s)::little-32, s::binary>> end),
    ])
  end

  defp binary_deserialize(data) do
    <<bs, bd, js, jd, n::little-32, rest::binary>> = data
    {langs, _} = take_strs(rest, n, [])
    langs = if bd != 0, do: langs, else: to_upper_self(langs)
    %{
      "binary_serialize" => bs != 0, "binary_deserialize" => bd != 0,
      "json_serialize" => js != 0, "json_deserialize" => jd != 0,
      "langs" => langs,
    }
  end

  defp take_strs(rest, 0, acc), do: {Enum.reverse(acc), rest}
  defp take_strs(<<slen::little-32, s::binary-size(slen), rest::binary>>, n, acc) do
    take_strs(rest, n - 1, [s | acc])
  end

  # --- json ------------------------------------------------------------------

  # Go's encoding/json emits struct fields in declaration order, and an Elixir
  # map has no order to emit — handing this map to Jason puts the keys in the
  # map's internal order, which is not the reference one. So the object is
  # built by hand in schema order.
  defp json_serialize(fm) do
    langs = if fm["json_serialize"], do: fm["langs"], else: to_upper_self(fm["langs"])
    IO.iodata_to_binary([
      "{\"binary_serialize\":#{fm["binary_serialize"]}",
      ",\"binary_deserialize\":#{fm["binary_deserialize"]}",
      ",\"json_serialize\":#{fm["json_serialize"]}",
      ",\"json_deserialize\":#{fm["json_deserialize"]}",
      ",\"langs\":[",
      Enum.map_join(langs, ",", &Jason.encode!/1),
      "]}",
    ])
  end

  defp json_deserialize(data) do
    d = Jason.decode!(data)
    langs = if d["json_deserialize"], do: d["langs"], else: to_upper_self(d["langs"])
    %{d | "langs" => langs}
  end

  # --- fault formats ---------------------------------------------------------

  defp err_ser(_fm), do: raise("injected serialize error")
  defp err_deser(_data), do: raise("injected deserialize error")
  defp hang_ser(fm) do
    Process.sleep(3000)
    binary_serialize(fm)
  end
  defp crash_ser(_fm) do
    System.halt(3)
  end
end
