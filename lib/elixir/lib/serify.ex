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

defmodule WorkerLib do
  @moduledoc """
  WorkerLib - Elixir implementation of the cross-language serialization test framework.

  FieldMap is a plain Elixir map with string keys.
  Struct fields are nested maps. list<struct> fields are lists of maps.

  ## Worker setup

  Build the worker as an escript and pass `-noinput` to the emulator:

      escript: [main_module: MyWorker, emu_args: "-noinput"]

  That hands fd 0 to this library instead of to the standard input device,
  which is what makes the NDJSON stream byte-exact on OTP 26 and older. Without
  it the worker still runs, but on those releases every control character in the
  case data is silently eaten before it arrives.
  """

  # The protocol revision this library speaks. The runner requires an exact
  # match and refuses to start a worker reporting anything else.
  @protocol_version 2

  defmodule Variant do
    @moduledoc """
    One arm of a sum: a tag and its decoded payload (nil for a unit variant).
    A sum field holds a `%Variant{}`.
    """
    defstruct [:tag, value: nil]
  end

  defmodule Type do
    @moduledoc """
    One data type: a model, and the formats whose functions speak it.

    `model` is the module the format functions take and return — anything with
    `from_field_map/1` and `to_field_map/1`, which `use WorkerLib.Serify.Model`
    generates. serify converts on either side, so the field map never reaches
    the worker:

        %Type{model: LedgerEntry, formats: %{
          "binary" => {&LedgerEntry.marshal/1, &LedgerEntry.unmarshal/1}}}

    `model: nil` is the other path, for a type with no natural struct (the
    audit fixtures mutate a field map on purpose): the functions then take and
    return the field map itself.
    """
    defstruct [:model, formats: %{}]
  end

  # run/2

  @doc """
  Single-type worker: handles whatever type/format the runner asks for.
  """
  def run(serialize_fn, deserialize_fn) do
    run_suite(%{"*" => %{"*" => {serialize_fn, deserialize_fn}}})
  end

  @doc """
  Multi-type worker. `suite` maps a type name to a `%WorkerLib.Type{}` — which
  carries the model its format functions speak — or to the older format name ->
  `{serialize, deserialize}` map, which still works and is what a type with no
  natural struct wants. A (type, format) that is not registered is reported to
  the runner as SKIPPED rather than failing the run.
  """
  def run_suite(suite) do
    loop(suite, open_input(), nil, nil, [], false)
  end

  @doc """
  Look one (type, format) up across both registration spellings, returning the
  field-map-level `{serialize, deserialize}` the run loop calls, or nil. `"*"`
  matches any type/format and backs the single-type `run/2` above.

  Public, and separate from the run loop, so it can be tested without stdin: an
  unresolved (type, format) is reported SKIPPED, so a registration shape this
  silently fails to understand produces a *green* conformance run made entirely
  of SKIPs — indistinguishable from a worker that honestly does not implement
  the type. In Elixir that is not a hypothetical: a struct *is* a map, so the
  `%Type{}` clause below has to come before the `is_map` one or every
  model-registered type falls through it into a `Map.get(%Type{}, "binary")`
  that answers nil.
  """
  def resolve_registered(suite, type, format) do
    case Map.get(suite, type) || Map.get(suite, "*") do
      %Type{} = t -> resolve_type(t, format)
      formats when is_map(formats) -> Map.get(formats, format) || Map.get(formats, "*")
      _ -> nil
    end
  end

  defp resolve_type(%Type{model: nil, formats: formats}, format) do
    Map.get(formats, format) || Map.get(formats, "*")
  end

  defp resolve_type(%Type{model: model, formats: formats}, format) do
    case Map.get(formats, format) || Map.get(formats, "*") do
      {ser, deser} ->
        {
          fn fm -> fm |> model.from_field_map() |> ser.() end,
          fn data -> data |> deser.() |> model.to_field_map() end
        }

      _ ->
        nil
    end
  end

  defp loop(suite, input, ser, deser, schema, audit_enabled) do
    case read_line(input) do
      :eof -> :ok
      {:error, _} -> :ok
      {line, input} ->
        line = String.trim(line)

        {ser, deser, schema, audit_enabled} =
          if line == "" do
            {ser, deser, schema, audit_enabled}
          else
            case Jason.decode(line) do
              {:ok, %{"op" => "ping"}} ->
                # Health check: report liveness and the protocol revision this
                # library speaks. Binds nothing.
                emit(%{"op" => "ping", "status" => "OK", "protocol_version" => @protocol_version})
                {ser, deser, schema, audit_enabled}

              {:ok, %{"op" => "bind"} = msg} ->
                handle_bind(suite, msg)

              {:ok, msg} ->
                {schema, audit_enabled} = handle(msg, schema, ser, deser, audit_enabled)
                {ser, deser, schema, audit_enabled}

              _ ->
                {ser, deser, schema, audit_enabled}
            end
          end

        loop(suite, input, ser, deser, schema, audit_enabled)
    end
  end

  # --- stdin ----------------------------------------------------------------
  #
  # NDJSON lines carry arbitrary case data, and on OTP 26 and older the standard
  # input device is not byte-transparent: every C0 control character is dropped
  # on its way through the input driver, and 0x08/0x7f additionally erase the
  # character before them (they are read as backspace). A case whose string
  # holds `del:` therefore reaches the worker as `del`, and the worker
  # serializes two bytes fewer than every other language. OTP 27 reads the same
  # bytes untouched, so the corruption only shows up on older runtimes.
  #
  # Reading fd 0 through a port instead of the standard input device gets the
  # bytes verbatim on every release, but the port can only have the descriptor
  # if the emulator has not already claimed it — hence `-noinput`, which
  # `escript: [emu_args: "-noinput"]` in the worker's mix.exs supplies. Without
  # it we fall back to the standard device, which is byte-exact from OTP 27 on.
  defp open_input do
    case :init.get_argument(:noinput) do
      {:ok, _} -> {Port.open({:fd, 0, 1}, [:binary, :in, :eof]), ""}
      _ -> warn_unless_byte_transparent()
    end
  end

  defp warn_unless_byte_transparent do
    release = List.to_string(:erlang.system_info(:otp_release))

    if String.to_integer(release) < 27 do
      IO.puts(
        :stderr,
        "serify: reading stdin through the standard input device on OTP #{release}, " <>
          "which drops control characters. Add escript: [emu_args: \"-noinput\"] " <>
          "to mix.exs for byte-exact input."
      )
    end

    :stdio
  end

  defp read_line(:stdio) do
    case IO.read(:line) do
      :eof -> :eof
      {:error, _} = err -> err
      line -> {line, :stdio}
    end
  end

  defp read_line({port, buf}) do
    case :binary.split(buf, "\n") do
      [line, rest] ->
        {line, {port, rest}}

      [^buf] when port == :closed and buf == "" ->
        :eof

      # Last line, unterminated: hand it over, then report eof on the next call.
      [^buf] when port == :closed ->
        {buf, {:closed, ""}}

      [^buf] ->
        receive do
          {^port, {:data, data}} -> read_line({port, buf <> data})
          {^port, :eof} -> read_line({:closed, buf})
        end
    end
  end

  defp handle_bind(suite, msg) do
    schema = parse_schema_fields(Map.get(msg, "schema", []))
    audit_enabled = Map.get(msg, "audit", false)

    pair = resolve_registered(suite, Map.get(msg, "type", ""), Map.get(msg, "format", ""))

    case pair do
      {ser, deser} ->
        emit(%{"op" => "bind"})
        {ser, deser, schema, audit_enabled}

      _ ->
        emit(%{"op" => "bind", "status" => "SKIPPED"})
        {nil, nil, schema, audit_enabled}
    end
  end

  # --- audit helpers --------------------------------------------------------

  @doc """
  This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
  """
  def collect_bin_snaps(fm, snaps) when is_map(fm) do
    keys = Map.keys(fm) |> Enum.sort()
    Enum.reduce(keys, snaps, fn k, acc ->
      case Map.get(fm, k) do
        v when is_binary(v) -> [{fm, k, v} | acc]
        v when is_map(v)    ->
          # Recurse into all values of the inner map too (handles map<K,struct>)
          Enum.reduce(v, acc, fn {_mk, mv}, a ->
            cond do
              is_map(mv) -> collect_bin_snaps(mv, a)
              is_binary(mv) -> [{fm, k, mv} | a]
              true -> a
            end
          end) |> then(fn a -> collect_bin_snaps(v, a) end)
        v when is_list(v)   -> Enum.reduce(v, acc, fn item, a ->
          if is_map(item), do: collect_bin_snaps(item, a), else: a
        end)
        _ -> acc
      end
    end)
  end

  @doc """
  This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
  """
  # `after` is a reserved word in Elixir and cannot be a parameter name.
  def dict_diffs(before, after_fm) do
    keys = MapSet.new(Map.keys(before)) |> MapSet.union(MapSet.new(Map.keys(after_fm)))
    keys |> Enum.filter(fn k ->
      Map.get(before, k) != Map.get(after_fm, k)
    end) |> Enum.sort()
  end

  @doc """
  This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
  """
  def detect_zero_copy(fm, buf) do
    if byte_size(buf) == 0, do: [], else: do_detect_zero_copy(fm, buf, collect_bin_snaps(fm, []))
  end

  defp do_detect_zero_copy(_fm, _buf, snaps) do
    # Other libraries detect zero-copy by XOR-flipping the input buffer and seeing
    # whether the decoded FieldMap changes with it. Elixir binaries are immutable,
    # so a decoded value can never alias the input buffer and there is nothing to
    # flip: the check reduces to comparing the snapshots.
    aliased = Enum.filter(snaps, fn {target_fm, key, orig} ->
      case Map.get(target_fm, key) do
        v when is_binary(v) -> v != orig
        _ -> false
      end
    end) |> Enum.map(fn {_fm, key, _orig} -> key end) |> Enum.uniq()

    # Restore
    Enum.each(snaps, fn {target_fm, key, orig} ->
      Map.put(target_fm, key, orig)
    end)

    aliased
  end

  # --- protocol handlers ----------------------------------------------------

  defp handle(%{"op" => "serialize", "id" => id, "data" => data}, schema, ser, _deser, audit_enabled) do
    result =
      try do
        fm = decode_field_map(data, schema)

        before = if audit_enabled, do: encode_field_map(fm, schema), else: nil

        bytes = ser.(fm)
        hex = Base.encode16(bytes, case: :lower)

        audit = if audit_enabled do
          a = %{}

          # Mutation: compare before/after
          after_map = encode_field_map(fm, schema)
          diffs = dict_diffs(before, after_map)
          a = if diffs != [], do: Map.put(a, "mutations", diffs), else: a

          # Output zero-copy is not detectable (or possible) on the BEAM:
          # binaries are immutable, so the output cannot mutably alias model
          # fields and there is no in-place flip to observe.

          # Stability: serialize again
          a = try do
            bytes2 = ser.(fm)
            if hex != Base.encode16(bytes2, case: :lower) do
              Map.put(a, "stable", false)
            else
              a
            end
          rescue
            _ -> Map.put(a, "stable", false)
          end

          a
        else
          nil
        end

        {:ok, hex, audit}
      rescue
        e -> {:error, Exception.message(e)}
      end
    case result do
      {:ok, hex, audit} ->
        resp = %{"id" => id, "op" => "serialize", "status" => "OK", "hex" => hex}
        resp = if audit != nil and map_size(audit) > 0, do: Map.put(resp, "audit", audit), else: resp
        emit(resp)
      {:error, e} -> emit(%{"id" => id, "op" => "serialize", "status" => "ERROR", "error" => e})
    end
    {schema, audit_enabled}
  end

  defp handle(%{"op" => "deserialize", "id" => id, "hex" => hex}, schema, _ser, deser, audit_enabled) do
    result =
      try do
        bytes = Base.decode16!(hex, case: :mixed)
        buf_snapshot = if audit_enabled, do: bytes, else: nil

        fm = deser.(bytes)
        data = encode_field_map(fm, schema)

        audit = if audit_enabled do
          a = %{}

          # Input-buffer mutation
          a = if buf_snapshot != bytes do
            Map.put(a, "input_mutated", true)
          else
            a
          end

          # Deser-stability (binaries are immutable — no clone needed)
          a = try do
            fm2 = deser.(buf_snapshot)
            data2 = encode_field_map(fm2, schema)
            if data2 == data, do: a, else: Map.put(a, "deser_stable", false)
          rescue
            _ -> Map.put(a, "deser_stable", false)
          end

          # Zero-copy
          zc = detect_zero_copy(fm, bytes)
          if zc != [], do: Map.put(a, "zero_copy_fields", zc), else: a
        else
          nil
        end

        {:ok, data, audit}
      rescue
        e -> {:error, Exception.message(e)}
      end
    case result do
      {:ok, d, audit} ->
        resp = %{"id" => id, "op" => "deserialize", "status" => "OK", "data" => d}
        resp = if audit != nil and map_size(audit) > 0, do: Map.put(resp, "audit", audit), else: resp
        emit(resp)
      {:error, e} -> emit(%{"id" => id, "op" => "deserialize", "status" => "ERROR", "error" => e})
    end
    {schema, audit_enabled}
  end

  defp handle(%{"op" => "exit"}, _schema, _ser, _deser, _audit), do: System.halt(0)
  defp handle(_msg, schema, _ser, _deser, audit_enabled), do: {schema, audit_enabled}

  # Schema parsing

  defp parse_schema_fields(raw_fields) do
    Enum.map(raw_fields, fn f ->
      %{
        name:     f["name"],
        type:     f["type"],
        fields:   parse_schema_fields(Map.get(f, "fields", [])),
        variants: parse_schema_variants(Map.get(f, "variants", [])),
        tags:     Map.get(f, "tags", %{})
      }
    end)
  end

  # One arm of a sum: a tag and its payload schema (nil for a unit variant).
  defp parse_schema_variants(raw_variants) do
    Enum.map(raw_variants || [], fn v ->
      payload =
        case v["payload"] do
          nil -> nil
          # Reuse the field parser by wrapping the payload in a one-element list.
          p -> parse_schema_fields([p]) |> hd()
        end

      %{name: v["name"], payload: payload}
    end)
  end

  defp find_schema_variant(sf, tag) do
    case Enum.find(Map.get(sf, :variants, []), &(&1.name == tag)) do
      nil -> raise ArgumentError, ~s(unknown variant "#{tag}")
      sv  -> sv
    end
  end

  # Decode

  def decode_field_map(data, schema) do
    Enum.reduce(schema, %{}, fn sf, acc ->
      case Map.fetch(data, sf.name) do
        {:ok, v} -> Map.put(acc, sf.name, decode_field(sf, v))
        :error   -> acc
      end
    end)
  end

  defp decode_field(%{type: "uint8"},  v), do: trunc(v)
  defp decode_field(%{type: "uint16"}, v), do: trunc(v)
  defp decode_field(%{type: "uint32"}, v), do: trunc(v)
  defp decode_field(%{type: t}, v) when t in ["uint64", "uint128"], do: String.to_integer(v)
  defp decode_field(%{type: "int8"},  v), do: trunc(v)
  defp decode_field(%{type: "int16"}, v), do: trunc(v)
  defp decode_field(%{type: "int32"}, v), do: trunc(v)
  defp decode_field(%{type: t}, v) when t in ["int64", "int128"], do: String.to_integer(v)
  defp decode_field(%{type: "float32"}, v) do
    <<f::little-float-32>> = Base.decode16!(v, case: :mixed)
    f
  end
  defp decode_field(%{type: "float64"}, v) do
    <<d::little-float-64>> = Base.decode16!(v, case: :mixed)
    d
  end
  defp decode_field(%{type: "bool"},   v), do: v
  defp decode_field(%{type: "string"}, v), do: v
  defp decode_field(%{type: "bytes"},  v), do: Base.decode16!(v, case: :mixed)
  defp decode_field(%{type: "struct", fields: nested_schema}, v) do
    decode_field_map(v, nested_schema)
  end
  defp decode_field(%{type: "list<string>"}, v), do: v
  defp decode_field(%{type: "optional<string>"}, nil), do: nil
  defp decode_field(%{type: "optional<string>"}, v),   do: v
  # An array<T,N> is a list whose length the schema fixes, so it shares the
  # list clause outright and adds only the length check. Matching the literal
  # type string "array<uint32,4>" is what pinned arrays to that one shape —
  # any other element type or length had no clause at all.
  defp decode_field(%{type: "array<" <> rest} = sf, v) do
    {elem, n} = split_array(rest)
    out = decode_field(%{sf | type: "list<#{elem}>"}, v)

    if length(out) != n do
      raise ArgumentError, "array: expected #{n} elements, got #{length(out)}"
    end

    out
  end
  defp decode_field(%{type: "list<struct>", fields: nested_schema}, v) do
    Enum.map(v, fn item -> decode_field_map(item, nested_schema) end)
  end
  defp decode_field(%{type: "optional<struct>", fields: _nested_schema}, nil), do: nil
  defp decode_field(%{type: "optional<struct>", fields: nested_schema}, v) do
    decode_field_map(v, nested_schema)
  end
  defp decode_field(%{type: "optional<" <> rest}, v) do
    inner = String.trim_trailing(rest, ">")
    if v == nil do
      nil
    else
      decode_field(%{type: inner, fields: []}, v)
    end
  end
  defp decode_field(%{type: "list<" <> rest}, v) do
    inner = String.trim_trailing(rest, ">")
    Enum.map(v, fn item -> decode_field(%{type: inner, fields: []}, item) end)
  end
  # enum<a,b,c>: the variant name travels as a string.
  defp decode_field(%{type: "enum<" <> _rest}, v), do: v

  # sum<a, b: T>: {tag: payload} on the wire (payload null for a unit variant);
  # the payload is decoded through the variant's own schema.
  defp decode_field(%{type: "sum<" <> _rest} = sf, v) do
    case Map.to_list(v) do
      [{tag, payload}] ->
        case find_schema_variant(sf, tag).payload do
          nil -> %Variant{tag: tag}
          psf -> %Variant{tag: tag, value: decode_field(psf, payload)}
        end

      other ->
        raise ArgumentError, "sum must name exactly one variant, got #{length(other)}"
    end
  end

  defp decode_field(%{type: "map<" <> rest} = sf, v) do
    {_key_type, val_type} = split_map_types("map<#{rest}")
    nested = Map.get(sf, :fields, [])
    Map.new(v, fn {k, item} ->
      {k,
        cond do
          val_type == "struct" -> decode_field_map(item, nested)
          val_type in ["uint64", "uint128"] -> String.to_integer(item)
          val_type in ["int64", "int128"] -> String.to_integer(item)
          val_type == "float32" ->
            <<f::little-float-32>> = Base.decode16!(item, case: :mixed); f
          val_type == "float64" ->
            <<d::little-float-64>> = Base.decode16!(item, case: :mixed); d
          val_type == "bytes" -> Base.decode16!(item, case: :mixed)
          val_type == "bool" -> item
          val_type == "string" -> item
          true -> item
        end
      }
    end)
  end
  defp decode_field(_sf, v), do: v

  # Split the inside of "array<T,N>" into its element type and length.
  defp split_array(rest) do
    inner = String.trim_trailing(rest, ">")
    {elem, count} = inner |> String.split(",") |> Enum.split(-1)
    {elem |> Enum.join(",") |> String.trim(), count |> hd() |> String.trim() |> String.to_integer()}
  end

  defp split_map_types(typ) do
    inner = String.trim_leading(typ, "map<") |> String.trim_trailing(">")
    {key_type, val_type} = split_on_comma(inner, 0, 0)
    {String.trim(key_type), String.trim(val_type)}
  end

  defp split_on_comma(s, _depth, idx) when idx >= byte_size(s), do: {s, ""}
  defp split_on_comma(s, depth, idx) do
    case String.at(s, idx) do
      ch when ch in ["<", "["] -> split_on_comma(s, depth + 1, idx + 1)
      ch when ch in [">", "]"] -> split_on_comma(s, depth - 1, idx + 1)
      "," when depth == 0 ->
        {String.slice(s, 0, idx), String.slice(s, idx + 1, byte_size(s) - idx - 1)}
      _ -> split_on_comma(s, depth, idx + 1)
    end
  end

  # Encode

  def encode_field_map(fm, schema) do
    Enum.reduce(schema, %{}, fn sf, acc ->
      case Map.fetch(fm, sf.name) do
        {:ok, v} -> Map.put(acc, sf.name, encode_field(sf, v))
        :error   -> acc
      end
    end)
  end

  defp encode_field(%{type: t}, v) when t in ["uint8","uint16","uint32","int8","int16","int32"], do: v
  defp encode_field(%{type: t}, v) when t in ["uint64","uint128","int64","int128"], do: Integer.to_string(v)
  # The wire format is the IEEE bits as little-endian hex; encode the packed
  # little-endian binary directly (re-packing the bits big-endian reverses it).
  defp encode_field(%{type: "float32"}, v), do: Base.encode16(<<v::little-float-32>>, case: :lower)
  defp encode_field(%{type: "float64"}, v), do: Base.encode16(<<v::little-float-64>>, case: :lower)
  defp encode_field(%{type: "bool"},   v), do: v
  defp encode_field(%{type: "string"}, v), do: v
  defp encode_field(%{type: "bytes"},  v), do: Base.encode16(v, case: :lower)
  defp encode_field(%{type: "struct", fields: nested_schema}, v) do
    encode_field_map(v, nested_schema)
  end
  defp encode_field(%{type: "list<string>"}, v), do: v
  defp encode_field(%{type: "optional<string>"}, v), do: v
  defp encode_field(%{type: "array<" <> rest} = sf, v) do
    {elem, _n} = split_array(rest)
    encode_field(%{sf | type: "list<#{elem}>"}, v)
  end
  defp encode_field(%{type: "list<struct>", fields: nested_schema}, v) do
    Enum.map(v, fn item -> encode_field_map(item, nested_schema) end)
  end
  defp encode_field(%{type: "optional<struct>", fields: _nested_schema}, nil), do: nil
  defp encode_field(%{type: "optional<struct>", fields: nested_schema}, v) do
    encode_field_map(v, nested_schema)
  end
  defp encode_field(%{type: "optional<" <> rest}, v) do
    inner = String.trim_trailing(rest, ">")
    if v == nil, do: nil, else: encode_field(%{type: inner, fields: []}, v)
  end
  defp encode_field(%{type: "list<" <> rest}, v) do
    inner = String.trim_trailing(rest, ">")
    Enum.map(v, fn item -> encode_field(%{type: inner, fields: []}, item) end)
  end
  defp encode_field(%{type: "enum<" <> _rest}, v), do: v

  # Inverse of the sum decode clause: a %Variant{} becomes {tag: payload}.
  defp encode_field(%{type: "sum<" <> _rest} = sf, %Variant{} = var) do
    case find_schema_variant(sf, var.tag).payload do
      nil -> %{var.tag => nil}
      psf -> %{var.tag => encode_field(psf, var.value)}
    end
  end

  defp encode_field(%{type: "map<" <> rest} = sf, v) do
    {_key_type, val_type} = split_map_types("map<#{rest}")
    nested = Map.get(sf, :fields, [])
    v
    |> Enum.sort_by(fn {k, _} -> k end)
    |> Map.new(fn {k, item} ->
      encoded = cond do
        val_type == "struct" -> encode_field_map(item, nested)
        val_type in ["uint64", "uint128", "int64", "int128"] -> Integer.to_string(item)
        val_type == "float32" ->
          Base.encode16(<<item::little-float-32>>, case: :lower)
        val_type == "float64" ->
          Base.encode16(<<item::little-float-64>>, case: :lower)
        val_type == "bytes" -> Base.encode16(item, case: :lower)
        true -> item
      end
      {k, encoded}
    end)
  end
  defp encode_field(_sf, v), do: v

  # Emit

  defp emit(obj) do
    IO.puts(Jason.encode!(obj))
  end

end

# ─── Serify.Model — compile-time schema macro ───────────────────────────

defmodule WorkerLib.Serify.Model do
  @moduledoc """
  Ecto-style schema for Serify models.

  Usage:
      defmodule MyApp.User do
        use WorkerLib.Serify.Model

        serify_field :user_id, :u64
        serify_field :name,    :string
        serify_field :email,   :string, key: "email_addr"
        serify_field :address, :struct, module: Address
        serify_field :labels,  :map, of: :string
        serify_field :scores,  :map, of: {:struct, GameScore}
      end

  This generates:
    - to_field_map/1  (struct → map)
    - from_field_map/1  (map → struct)
  """

  defmacro __using__(_opts) do
    quote do
      import unquote(__MODULE__)
      Module.register_attribute(__MODULE__, :serify_fields, accumulate: true)
      @before_compile unquote(__MODULE__)
    end
  end

  # Must return the generated AST directly. Wrapping the call in `quote` would
  # merely evaluate it inside the user's module body and throw the AST away,
  # defining nothing.
  defmacro __before_compile__(env) do
    env.module
    |> Module.get_attribute(:serify_fields)
    |> Enum.reverse()
    |> def_serify_functions(env.module)
  end

  defmacro serify_field(name, type, opts \\ []) do
    quote do
      @serify_fields {unquote(name), unquote(type), unquote(opts)}
    end
  end

  # Generate to_field_map / from_field_map at compile time.
  def def_serify_functions(fields, _mod) do
    # Generate to_field_map clauses
    to_clauses =
      for {name, type, opts} <- fields do
        key = Keyword.get(opts, :key, to_string(name))
        to_clause(name, type, key, opts)
      end

    from_clauses =
      for {name, type, opts} <- fields do
        key = Keyword.get(opts, :key, to_string(name))
        from_clause(name, type, key, opts)
      end

    quote do
      def to_field_map(struct) do
        fm = %{}
        unquote_splicing(to_clauses)
        fm
      end

      def from_field_map(fm) do
        struct = %__MODULE__{}
        unquote_splicing(from_clauses)
        struct
      end
    end
  end

  # ── to_field_map helpers ──────────────────────────────────────────────

  defp to_clause(name, :u8, key, _opts),     do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :u16, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :u32, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :u64, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :i8, key, _opts),     do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :i16, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :i32, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :i64, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :u128, key, _opts),   do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :i128, key, _opts),   do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :f32, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :f64, key, _opts),    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :bool, key, _opts),   do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :string, key, _opts), do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, :bytes, key, _opts),  do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, {:list, :struct}, key, opts) do
    mod = Keyword.get(opts, :module)
    quote do
      fm = Map.put(fm, unquote(key),
        Enum.map(struct.unquote(name), &unquote(mod).to_field_map/1))
    end
  end
  # Every other element type: the FieldMap carries the list as-is and the schema
  # decides the element's wire form, so one clause covers all of them. Naming
  # them individually is what left :u16, :i8, :bool, :bytes and the rest unbound.
  defp to_clause(name, {:list, _elem}, key, _opts),
    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  # An array<T,N> is carried exactly as the list<T> it is.
  defp to_clause(name, {:array, _elem}, key, _opts),
    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, {:optional, :string}, key, _opts),
    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  # Every other optional<T>: the FieldMap carries nil or the bare value, exactly
  # as it does for a string, so one clause covers all of them. Naming only
  # :string and :struct is what left optional<float32> and the rest unbound.
  defp to_clause(name, {:optional, elem}, key, _opts) when elem != :struct,
    do: quote(do: fm = Map.put(fm, unquote(key), struct.unquote(name)))
  defp to_clause(name, {:optional, :struct}, key, opts) do
    mod = Keyword.get(opts, :module)

    quote do
      # The `if` needs parentheses: as a bare nested call inside Map.put/3 it is
      # ambiguous and Elixir rejects it.
      fm =
        Map.put(
          fm,
          unquote(key),
          if(struct.unquote(name), do: unquote(mod).to_field_map(struct.unquote(name)))
        )
    end
  end
  defp to_clause(name, :struct, key, opts) do
    mod = Keyword.get(opts, :module)
    quote do
      fm = Map.put(fm, unquote(key), unquote(mod).to_field_map(struct.unquote(name)))
    end
  end
  defp to_clause(name, :sum, key, _opts) do
    quote do
      fm = Map.put(fm, unquote(key), WorkerLib.Serify.Model.variant_of(struct.unquote(name)))
    end
  end
  defp to_clause(name, :map, key, opts) do
    of = Keyword.get(opts, :of, :string)
    case of do
      {:struct, mod} ->
        quote do
          fm = Map.put(fm, unquote(key),
            Map.new(struct.unquote(name), fn {k, v} ->
              {k, unquote(mod).to_field_map(v)}
            end))
        end
      _ ->
        quote do
          fm = Map.put(fm, unquote(key), struct.unquote(name))
        end
    end
  end

  # sum: a tagged tuple is already a tagged union
  #
  # Elixir has no sum type to declare, and needs none — `{:sms, "hi"}` *is* a tag
  # and a payload, and `:silent` on its own is a unit variant. So `serify_field(
  # :channel, :sum)` is the whole binding: no arm list, no converter, no
  # registration. The arity rule every other serify binding spells out falls out
  # of the shape:
  #
  #     :silent               -> a unit variant, no payload
  #     {:sms, "hi"}          -> that value is the payload
  #     {:invoice, %{...}}    -> a map payload is the struct holding N fields

  @doc false
  def variant_of(nil), do: raise(ArgumentError, "sum field is unset")
  def variant_of(tag) when is_atom(tag), do: %WorkerLib.Variant{tag: Atom.to_string(tag), value: nil}
  def variant_of({tag, value}) when is_atom(tag),
    do: %WorkerLib.Variant{tag: Atom.to_string(tag), value: value}

  def variant_of(other) do
    raise ArgumentError,
          "a sum must be an atom (unit variant) or a {tag, payload} tuple, got #{inspect(other)}"
  end

  @doc false
  # to_existing_atom, not to_atom: the worker's own code names every tag it can
  # handle, so those atoms exist by the time this runs — and an unknown tag from
  # the wire then raises instead of growing the atom table without bound.
  def variant_to_term(%WorkerLib.Variant{tag: tag, value: nil}), do: existing_tag(tag)
  def variant_to_term(%WorkerLib.Variant{tag: tag, value: value}), do: {existing_tag(tag), value}

  def variant_to_term(other),
    do: raise(ArgumentError, "expected a variant, got #{inspect(other)}")

  defp existing_tag(tag) do
    String.to_existing_atom(tag)
  rescue
    ArgumentError -> reraise ArgumentError, [message: "unknown variant #{inspect(tag)}"], __STACKTRACE__
  end

  # ── from_field_map helpers ────────────────────────────────────────────

  defp from_clause(name, :u8, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :u16, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :u32, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :u64, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :i8, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :i16, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :i32, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :i64, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :u128, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :i128, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0)))
  defp from_clause(name, :f32, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0.0)))
  defp from_clause(name, :f64, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), 0.0)))
  defp from_clause(name, :bool, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), false)))
  defp from_clause(name, :string, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), "")))
  defp from_clause(name, :bytes, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), <<>>)))
  defp from_clause(name, {:list, :struct}, key, opts) do
    mod = Keyword.get(opts, :module)
    quote do
      struct = Map.put(struct, unquote(name),
        (Map.get(fm, unquote(key), []) || [])
        |> Enum.map(&unquote(mod).from_field_map/1))
    end
  end
  # Mirror of the to_clause catch-all: one clause for every non-struct element
  # type, since the FieldMap already holds the list in its decoded form.
  defp from_clause(name, {:list, _elem}, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), [])))
  defp from_clause(name, {:array, _elem}, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), [])))
  defp from_clause(name, {:optional, :string}, key, _opts),
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key))))
  defp from_clause(name, {:optional, elem}, key, _opts) when elem != :struct,
    do: quote(do: struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key))))
  defp from_clause(name, {:optional, :struct}, key, opts) do
    mod = Keyword.get(opts, :module)
    quote do
      struct = Map.put(struct, unquote(name),
        case Map.get(fm, unquote(key)) do
          nil -> nil
          v   -> unquote(mod).from_field_map(v)
        end)
    end
  end
  defp from_clause(name, :struct, key, opts) do
    mod = Keyword.get(opts, :module)
    quote do
      struct = Map.put(struct, unquote(name),
        unquote(mod).from_field_map(Map.get(fm, unquote(key), %{})))
    end
  end
  defp from_clause(name, :sum, key, _opts) do
    quote do
      struct =
        Map.put(struct, unquote(name),
          WorkerLib.Serify.Model.variant_to_term(Map.get(fm, unquote(key))))
    end
  end
  defp from_clause(name, :map, key, opts) do
    of = Keyword.get(opts, :of, :string)
    case of do
      {:struct, mod} ->
        quote do
          struct = Map.put(struct, unquote(name),
            (Map.get(fm, unquote(key), %{}) || %{})
            |> Map.new(fn {k, v} -> {k, unquote(mod).from_field_map(v)} end))
        end
      _ ->
        quote do
          struct = Map.put(struct, unquote(name), Map.get(fm, unquote(key), %{}))
        end
    end
  end
end

