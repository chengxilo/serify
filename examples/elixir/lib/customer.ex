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

defmodule Address do
  @moduledoc """
  Mirrors the reusable examples/cases/address.yaml, imported by customer and
  order.

  A struct is its fields back to back in schema order — nothing frames it — so
  pack/1 returns a fragment and unpack/1 takes the rest of the buffer and hands
  back what it did not consume.
  """
  use WorkerLib.Serify.Model

  defstruct recipient: "", street: "", city: "", country: "", postal_code: ""

  serify_field(:recipient, :string)
  serify_field(:street, :string)
  serify_field(:city, :string)
  serify_field(:country, :string)
  serify_field(:postal_code, :string)

  def pack(%__MODULE__{} = a) do
    Customer.Wire.str(a.recipient) <>
      Customer.Wire.str(a.street) <>
      Customer.Wire.str(a.city) <>
      Customer.Wire.str(a.country) <> Customer.Wire.str(a.postal_code)
  end

  def unpack(data) do
    {recipient, data} = Customer.Wire.take_str(data)
    {street, data} = Customer.Wire.take_str(data)
    {city, data} = Customer.Wire.take_str(data)
    {country, data} = Customer.Wire.take_str(data)
    {postal_code, data} = Customer.Wire.take_str(data)

    {%__MODULE__{
       recipient: recipient,
       street: street,
       city: city,
       country: country,
       postal_code: postal_code
     }, data}
  end

  def to_json(%__MODULE__{} = a) do
    %{
      "recipient" => a.recipient,
      "street" => a.street,
      "city" => a.city,
      "country" => a.country,
      "postal_code" => a.postal_code
    }
  end

  def from_json(o) do
    %__MODULE__{
      recipient: o["recipient"],
      street: o["street"],
      city: o["city"],
      country: o["country"],
      postal_code: o["postal_code"]
    }
  end
end

defmodule Money do
  @moduledoc "Mirrors the reusable examples/cases/money.yaml, imported by customer and order."
  use WorkerLib.Serify.Model

  defstruct currency: "", amount_minor: 0

  serify_field(:currency, :string)
  serify_field(:amount_minor, :i64)

  def pack(%__MODULE__{} = m) do
    Customer.Wire.str(m.currency) <> <<m.amount_minor::signed-little-64>>
  end

  def unpack(data) do
    {currency, <<amount_minor::signed-little-64, rest::binary>>} = Customer.Wire.take_str(data)
    {%__MODULE__{currency: currency, amount_minor: amount_minor}, rest}
  end

  def to_json(%__MODULE__{} = m),
    do: %{"currency" => m.currency, "amount_minor" => m.amount_minor}

  def from_json(o), do: %__MODULE__{currency: o["currency"], amount_minor: o["amount_minor"]}
end

defmodule Customer.Wire do
  @moduledoc """
  The two length-prefixed primitives customer needs, in the take/rest shape the
  nested structs above are written against. ledger.ex and signals.ex match their
  whole layout in one bitstring pattern instead, which customer's four
  variable-length collections rule out.
  """

  @doc "u32 byte length, then the UTF-8 bytes."
  def str(s), do: <<byte_size(s)::little-32, s::binary>>

  @doc "Read one length-prefixed string, returning it and the rest of the buffer."
  def take_str(<<n::little-32, s::binary-size(n), rest::binary>>), do: {s, rest}

  @doc "Read `n` items with `fun`, returning them in order and the rest of the buffer."
  def take_n(data, n, fun), do: take_n(data, n, fun, [])

  defp take_n(data, 0, _fun, acc), do: {Enum.reverse(acc), data}

  defp take_n(data, n, fun, acc) do
    {item, rest} = fun.(data)
    take_n(rest, n - 1, fun, [item | acc])
  end
end

defmodule CustomerRecord do
  @moduledoc """
  Mirrors examples/cases/customer.yaml — a store account.

  This is the only type in the suite carrying two formats, and the second one is
  the point: `binary` is a layout written by hand below, `json` goes through
  OTP 27's `:json`, so the two fail in completely different ways. Both declare
  `oracle: semantic`, so what has to match the reference is the decoded value
  rather than the bytes — Go's encoder HTML-escapes `<`, `>` and `&` and always
  escapes U+2028/U+2029, and under a byte oracle every worker would have to
  reproduce that quirk.

  It is also this worker's first model with nested structs, and the binding
  takes them without help: `:struct` names the module, `{:list, :struct}` and
  `{:map, {:struct, _}}` name it for their elements.

  Nothing here needs the float care telemetry does. customer's only float is
  `fraud_score`, whose cases stay inside what a BEAM float can hold — the NaN
  and infinity that keep telemetry out of this worker never appear.

  Go is the --ref language and owns the byte layout; see examples/go/wire.go.
  """
  use WorkerLib.Serify.Model

  alias Customer.Wire

  defstruct customer_id: 0,
            email: "",
            display_name: "",
            age: 0,
            email_verified: false,
            fraud_score: 0.0,
            loyalty_points: 0,
            signup_ts: 0,
            avatar_sha256: "",
            pin: [],
            referral_code: nil,
            store_credit: %Money{},
            shipping_addresses: [],
            address_book: %{},
            wishlist_skus: [],
            preferences: %{}

  serify_field(:customer_id, :u64)
  serify_field(:email, :string)
  serify_field(:display_name, :string)
  serify_field(:age, :u8)
  serify_field(:email_verified, :bool)
  serify_field(:fraud_score, :f32)
  serify_field(:loyalty_points, :u32)
  serify_field(:signup_ts, :i64)
  serify_field(:avatar_sha256, :bytes)
  serify_field(:pin, {:array, :u8})
  serify_field(:referral_code, {:optional, :string})
  serify_field(:store_credit, :struct, module: Money)
  serify_field(:shipping_addresses, {:list, :struct}, module: Address)
  serify_field(:address_book, :map, of: {:struct, Address})
  serify_field(:wishlist_skus, {:list, :string})
  serify_field(:preferences, :map, of: :string)

  def marshal(%__MODULE__{} = c) do
    verified = if c.email_verified, do: 1, else: 0

    # optional<string>: a presence flag, then the value if present. An empty
    # string is present, which is why the flag cannot be inferred from it.
    referral =
      case c.referral_code do
        nil -> <<0>>
        s -> <<1>> <> Wire.str(s)
      end

    # Entry order is the map's own — deliberately not sorted. A map is
    # unordered, so customer declares `oracle: semantic` and the decoded value
    # is what gets compared. See docs/protocol.md.
    book =
      Enum.map_join(c.address_book, fn {k, a} -> Wire.str(k) <> Address.pack(a) end)

    prefs = Enum.map_join(c.preferences, fn {k, v} -> Wire.str(k) <> Wire.str(v) end)

    # array<T,N> carries no count: N is fixed by the schema.
    <<c.customer_id::little-64>> <>
      Wire.str(c.email) <>
      Wire.str(c.display_name) <>
      <<c.age::8, verified::8, c.fraud_score::little-float-32, c.loyalty_points::little-32,
        c.signup_ts::signed-little-64, byte_size(c.avatar_sha256)::little-32,
        c.avatar_sha256::binary>> <>
      :erlang.list_to_binary(c.pin) <>
      referral <>
      Money.pack(c.store_credit) <>
      <<length(c.shipping_addresses)::little-32>> <>
      Enum.map_join(c.shipping_addresses, &Address.pack/1) <>
      <<map_size(c.address_book)::little-32>> <>
      book <>
      <<length(c.wishlist_skus)::little-32>> <>
      Enum.map_join(c.wishlist_skus, &Wire.str/1) <>
      <<map_size(c.preferences)::little-32>> <> prefs
  end

  def unmarshal(data) do
    <<customer_id::little-64, rest::binary>> = data
    {email, rest} = Wire.take_str(rest)
    {display_name, rest} = Wire.take_str(rest)

    <<age::8, verified::8, fraud_score::little-float-32, loyalty_points::little-32,
      signup_ts::signed-little-64, avatar_len::little-32, avatar_sha256::binary-size(avatar_len),
      pin::binary-size(4), has_referral::8, rest::binary>> = rest

    {referral_code, rest} =
      if has_referral == 0, do: {nil, rest}, else: Wire.take_str(rest)

    {store_credit, rest} = Money.unpack(rest)

    <<n_addrs::little-32, rest::binary>> = rest
    {shipping_addresses, rest} = Wire.take_n(rest, n_addrs, &Address.unpack/1)

    <<n_book::little-32, rest::binary>> = rest

    {book_pairs, rest} =
      Wire.take_n(rest, n_book, fn d ->
        {k, d} = Wire.take_str(d)
        {a, d} = Address.unpack(d)
        {{k, a}, d}
      end)

    <<n_skus::little-32, rest::binary>> = rest
    {wishlist_skus, rest} = Wire.take_n(rest, n_skus, &Wire.take_str/1)

    <<n_prefs::little-32, rest::binary>> = rest

    {pref_pairs, _rest} =
      Wire.take_n(rest, n_prefs, fn d ->
        {k, d} = Wire.take_str(d)
        {v, d} = Wire.take_str(d)
        {{k, v}, d}
      end)

    %__MODULE__{
      customer_id: customer_id,
      email: email,
      display_name: display_name,
      age: age,
      email_verified: verified != 0,
      fraud_score: fraud_score,
      loyalty_points: loyalty_points,
      signup_ts: signup_ts,
      avatar_sha256: avatar_sha256,
      pin: :erlang.binary_to_list(pin),
      referral_code: referral_code,
      store_credit: store_credit,
      shipping_addresses: shipping_addresses,
      address_book: Map.new(book_pairs),
      wishlist_skus: wishlist_skus,
      preferences: Map.new(pref_pairs)
    }
  end

  @doc """
  `bytes` is base64 and the 64-bit integers are JSON numbers: that is what the
  reference worker's `[]byte` and `uint64` marshal to, and the semantic oracle
  decodes our output with it. Elixir integers are arbitrary-precision, so max
  uint64 needs none of the care the other dynamic languages spend on it.
  """
  def to_json(%__MODULE__{} = c) do
    o = %{
      "customer_id" => c.customer_id,
      "email" => c.email,
      "display_name" => c.display_name,
      "age" => c.age,
      "email_verified" => c.email_verified,
      "fraud_score" => c.fraud_score,
      "loyalty_points" => c.loyalty_points,
      "signup_ts" => c.signup_ts,
      "avatar_sha256" => Base.encode64(c.avatar_sha256),
      "pin" => c.pin,
      # Erlang's :json spells JSON null as the atom :null and renders any other
      # atom as a string, so Elixir's nil would go out as "nil".
      "referral_code" => if(c.referral_code == nil, do: :null, else: c.referral_code),
      "store_credit" => Money.to_json(c.store_credit),
      "shipping_addresses" => Enum.map(c.shipping_addresses, &Address.to_json/1),
      "address_book" => Map.new(c.address_book, fn {k, a} -> {k, Address.to_json(a)} end),
      "wishlist_skus" => c.wishlist_skus,
      "preferences" => c.preferences
    }

    :erlang.iolist_to_binary(:json.encode(o))
  end

  def from_json(data) do
    o = :json.decode(data)

    %__MODULE__{
      customer_id: o["customer_id"],
      email: o["email"],
      display_name: o["display_name"],
      age: o["age"],
      email_verified: o["email_verified"],
      # A JSON number is a double; narrow it the way the wire does, so a float32
      # field holds a value float32 can actually represent.
      fraud_score: narrow_f32(o["fraud_score"]),
      loyalty_points: o["loyalty_points"],
      signup_ts: o["signup_ts"],
      avatar_sha256: Base.decode64!(o["avatar_sha256"]),
      pin: o["pin"],
      referral_code: unnull(o["referral_code"]),
      store_credit: Money.from_json(o["store_credit"]),
      shipping_addresses: Enum.map(o["shipping_addresses"], &Address.from_json/1),
      address_book: Map.new(o["address_book"], fn {k, a} -> {k, Address.from_json(a)} end),
      wishlist_skus: o["wishlist_skus"],
      preferences: o["preferences"]
    }
  end

  # The other half: :json.decode hands back :null, which is not Elixir's nil.
  defp unnull(:null), do: nil
  defp unnull(v), do: v

  # `:json` hands back an integer for a whole number, and 0 is one of
  # fraud_score's cases.
  defp narrow_f32(v) when is_integer(v), do: narrow_f32(v * 1.0)

  defp narrow_f32(v) do
    <<f::little-float-32>> = <<v::little-float-32>>
    f
  end
end
