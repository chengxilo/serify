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

defmodule HappyWorker do
  @moduledoc """
  Happy-path Elixir worker: `all_types` in binary and json.

  Go is the --ref language and owns both byte layouts; see
  test/cases/happy/go/type.go. The json format must match Go's encoding/json
  byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
  byte order (Elixir binary comparison), []byte as base64, floats in shortest
  form without a trailing .0, and U+2028/U+2029 escaped (Go escapes those
  unconditionally).
  """

  @status_variants ["pending", "paid", "shipped", "delivered", "cancelled"]

  def main(_args) do
    WorkerLib.run_suite(%{
      "all_types" => %{
        "binary" => {&binary_serialize/1, &binary_deserialize/1},
        "json" => {&json_serialize/1, &json_deserialize/1}
      }
    })
  end

  defp status_ordinal(s) do
    case Enum.find_index(@status_variants, &(&1 == s)) do
      nil -> raise "unknown status #{inspect(s)}"
      i -> i
    end
  end

  # --- binary format ---------------------------------------------------------

  defp len_str(s), do: <<byte_size(s)::little-32, s::binary>>

  defp binary_serialize(fm) do
    list = fm["list"]
    opt = case fm["optional"] do
      nil -> <<0>>
      s -> <<1, len_str(s)::binary>>
    end
    [a0, a1, a2, a3] = fm["array"]
    p = fm["struct"]
    m = fm["map"]
    ms = fm["map_struct"]

    map_bin =
      m
      |> Map.keys()
      |> Enum.sort()
      |> Enum.map(fn k -> <<len_str(k)::binary, m[k]::little-32>> end)
      |> IO.iodata_to_binary()

    ms_bin =
      ms
      |> Map.keys()
      |> Enum.sort()
      |> Enum.map(fn k ->
        t = ms[k]
        <<len_str(k)::binary, len_str(t["name"])::binary, t["weight"]::little-32>>
      end)
      |> IO.iodata_to_binary()

    <<
      fm["uint8"],
      fm["uint16"]::little-16,
      fm["uint32"]::little-32,
      fm["uint64"]::little-64,
      fm["int8"]::signed-little-8,
      fm["int16"]::signed-little-16,
      fm["int32"]::signed-little-32,
      fm["int64"]::signed-little-64,
      fm["float32"]::float-little-32,
      fm["float64"]::float-little-64,
      if(fm["bool"], do: 1, else: 0),
      len_str(fm["string"])::binary,
      byte_size(fm["bytes"])::little-32,
      fm["bytes"]::binary,
      length(list)::little-32,
      Enum.map_join(list, &len_str/1)::binary,
      opt::binary,
      a0::little-32, a1::little-32, a2::little-32, a3::little-32,
      p["x"]::signed-little-32,
      p["y"]::signed-little-32,
      p["z"]::signed-little-32,
      len_str(p["name"])::binary,
      map_size(m)::little-32,
      map_bin::binary,
      map_size(ms)::little-32,
      ms_bin::binary,
      status_ordinal(fm["status"])
    >>
  end

  defp take_str(<<n::little-32, rest::binary>>) do
    <<s::binary-size(n), rest::binary>> = rest
    {s, rest}
  end

  defp take_strs(rest, 0, acc), do: {Enum.reverse(acc), rest}

  defp take_strs(rest, n, acc) do
    {s, rest} = take_str(rest)
    take_strs(rest, n - 1, [s | acc])
  end

  defp take_map(rest, 0, acc), do: {acc, rest}

  defp take_map(rest, n, acc) do
    {k, rest} = take_str(rest)
    <<v::little-32, rest::binary>> = rest
    take_map(rest, n - 1, Map.put(acc, k, v))
  end

  defp take_map_struct(rest, 0, acc), do: {acc, rest}

  defp take_map_struct(rest, n, acc) do
    {k, rest} = take_str(rest)
    {name, rest} = take_str(rest)
    <<weight::little-32, rest::binary>> = rest
    take_map_struct(rest, n - 1, Map.put(acc, k, %{"name" => name, "weight" => weight}))
  end

  defp binary_deserialize(data) do
    <<
      u8,
      u16::little-16,
      u32::little-32,
      u64::little-64,
      i8::signed-little-8,
      i16::signed-little-16,
      i32::signed-little-32,
      i64::signed-little-64,
      f32::float-little-32,
      f64::float-little-64,
      bool_byte,
      rest::binary
    >> = data

    {string, rest} = take_str(rest)
    <<nbytes::little-32, bytes::binary-size(nbytes), rest::binary>> = rest
    <<nlist::little-32, rest::binary>> = rest
    {list, rest} = take_strs(rest, nlist, [])

    <<has_opt, rest::binary>> = rest

    {optional, rest} =
      if has_opt == 0 do
        {nil, rest}
      else
        take_str(rest)
      end

    <<a0::little-32, a1::little-32, a2::little-32, a3::little-32, rest::binary>> = rest
    <<px::signed-little-32, py::signed-little-32, pz::signed-little-32, rest::binary>> = rest
    {pname, rest} = take_str(rest)

    <<nmap::little-32, rest::binary>> = rest
    {m, rest} = take_map(rest, nmap, %{})
    <<nms::little-32, rest::binary>> = rest
    {ms, rest} = take_map_struct(rest, nms, %{})

    <<ord>> = rest

    status = Enum.at(@status_variants, ord) || raise "status ordinal #{ord} out of range"

    %{
      "uint8" => u8,
      "uint16" => u16,
      "uint32" => u32,
      "uint64" => u64,
      "int8" => i8,
      "int16" => i16,
      "int32" => i32,
      "int64" => i64,
      "float32" => f32,
      "float64" => f64,
      "bool" => bool_byte != 0,
      "string" => string,
      "bytes" => bytes,
      "list" => list,
      "optional" => optional,
      "array" => [a0, a1, a2, a3],
      "struct" => %{"x" => px, "y" => py, "z" => pz, "name" => pname},
      "map" => m,
      "map_struct" => ms,
      "status" => status
    }
  end

  # --- json format -------------------------------------------------------------

  # Go's encoding/json string escaping with SetEscapeHTML(false): only \n, \r,
  # \t are named (\b and \f become \u00xx escapes), and U+2028/U+2029 are
  # escaped unconditionally (UTF-8: E2 80 A8 / E2 80 A9).
  defp go_str(s), do: IO.iodata_to_binary([?", go_str_bytes(s, []), ?"])

  defp go_str_bytes(<<>>, acc), do: Enum.reverse(acc)

  defp go_str_bytes(<<0xE2, 0x80, c, rest::binary>>, acc) when c in [0xA8, 0xA9] do
    esc = if c == 0xA8, do: "\\u2028", else: "\\u2029"
    go_str_bytes(rest, [esc | acc])
  end

  defp go_str_bytes(<<?", rest::binary>>, acc), do: go_str_bytes(rest, ["\\\"" | acc])
  defp go_str_bytes(<<?\\, rest::binary>>, acc), do: go_str_bytes(rest, ["\\\\" | acc])
  defp go_str_bytes(<<?\n, rest::binary>>, acc), do: go_str_bytes(rest, ["\\n" | acc])
  defp go_str_bytes(<<?\r, rest::binary>>, acc), do: go_str_bytes(rest, ["\\r" | acc])
  defp go_str_bytes(<<?\t, rest::binary>>, acc), do: go_str_bytes(rest, ["\\t" | acc])

  defp go_str_bytes(<<c, rest::binary>>, acc) when c < 0x20 do
    esc = "\\u" <> String.pad_leading(Integer.to_string(c, 16), 4, "0")
    go_str_bytes(rest, [String.downcase(esc) | acc])
  end

  defp go_str_bytes(<<c, rest::binary>>, acc), do: go_str_bytes(rest, [<<c>> | acc])

  # Go prints floats in shortest round-trip form without a trailing .0.
  defp go_f64(v) do
    s = :erlang.float_to_binary(v, [:short])
    if String.ends_with?(s, ".0"), do: binary_part(s, 0, byte_size(s) - 2), else: s
  end

  # Shortest decimal that round-trips through float32 (v is the f64 widening).
  defp go_f32(v) do
    bits = <<v::float-little-32>>
    Enum.find_value(0..9, fn d ->
      s = :erlang.float_to_binary(v, decimals: d)
      {f, ""} = Float.parse(s)

      if <<f::float-little-32>> == bits do
        s
        |> String.replace(~r/\.?0+$/, "")
        |> case do
          "" -> "0"
          "-" -> "-0"
          out -> out
        end
      end
    end) || go_f64(v)
  end

  defp json_serialize(fm) do
    m = fm["map"]
    ms = fm["map_struct"]
    p = fm["struct"]

    map_json =
      m
      |> Map.keys()
      |> Enum.sort()
      |> Enum.map_join(",", fn k -> "#{go_str(k)}:#{m[k]}" end)

    ms_json =
      ms
      |> Map.keys()
      |> Enum.sort()
      |> Enum.map_join(",", fn k ->
        t = ms[k]
        "#{go_str(k)}:{\"name\":#{go_str(t["name"])},\"weight\":#{t["weight"]}}"
      end)

    IO.iodata_to_binary([
      "{\"uint8\":#{fm["uint8"]}",
      ",\"uint16\":#{fm["uint16"]}",
      ",\"uint32\":#{fm["uint32"]}",
      ",\"uint64\":#{fm["uint64"]}",
      ",\"int8\":#{fm["int8"]}",
      ",\"int16\":#{fm["int16"]}",
      ",\"int32\":#{fm["int32"]}",
      ",\"int64\":#{fm["int64"]}",
      ",\"float32\":#{go_f32(fm["float32"])}",
      ",\"float64\":#{go_f64(fm["float64"])}",
      ",\"bool\":#{fm["bool"]}",
      ",\"string\":#{go_str(fm["string"])}",
      ",\"bytes\":\"#{Base.encode64(fm["bytes"])}\"",
      ",\"list\":[#{Enum.map_join(fm["list"], ",", &go_str/1)}]",
      ",\"optional\":#{if fm["optional"] == nil, do: "null", else: go_str(fm["optional"])}",
      ",\"array\":[#{Enum.join(fm["array"], ",")}]",
      ",\"struct\":{\"x\":#{p["x"]},\"y\":#{p["y"]},\"z\":#{p["z"]},\"name\":#{go_str(p["name"])}}",
      ",\"map\":{#{map_json}}",
      ",\"map_struct\":{#{ms_json}}",
      ",\"status\":#{go_str(fm["status"])}}"
    ])
  end

  # Jason decodes integer literals as integers; float fields must come back as
  # floats (Go emits e.g. `0` for a zero float64).
  defp to_float(v) when is_integer(v), do: v * 1.0
  defp to_float(v), do: v

  defp json_deserialize(data) do
    v = Jason.decode!(data)

    %{
      "uint8" => v["uint8"],
      "uint16" => v["uint16"],
      "uint32" => v["uint32"],
      "uint64" => v["uint64"],
      "int8" => v["int8"],
      "int16" => v["int16"],
      "int32" => v["int32"],
      "int64" => v["int64"],
      "float32" => to_float(v["float32"]),
      "float64" => to_float(v["float64"]),
      "bool" => v["bool"],
      "string" => v["string"],
      "bytes" => Base.decode64!(v["bytes"]),
      "list" => v["list"],
      "optional" => v["optional"],
      "array" => v["array"],
      "struct" => v["struct"],
      "map" => v["map"],
      "map_struct" => v["map_struct"],
      "status" => v["status"]
    }
  end
end
