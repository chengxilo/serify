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

defmodule LineItem do
  @moduledoc """
  Mirrors the reusable examples/cases/line_item.yaml, imported by order.

  Its own `unit_price` is a money, which makes any type using it the suite's
  only struct-inside-struct.
  """
  use WorkerLib.Serify.Model

  alias Customer.Wire

  defstruct sku: "",
            product_name: "",
            quantity: 0,
            unit_price: %Money{},
            discount_pct: 0,
            gift_wrap: false

  serify_field(:sku, :string)
  serify_field(:product_name, :string)
  serify_field(:quantity, :u16)
  serify_field(:unit_price, :struct, module: Money)
  serify_field(:discount_pct, :u8)
  serify_field(:gift_wrap, :bool)

  def pack(%__MODULE__{} = it) do
    Wire.str(it.sku) <>
      Wire.str(it.product_name) <>
      <<it.quantity::little-16>> <>
      Money.pack(it.unit_price) <>
      <<it.discount_pct::8, if(it.gift_wrap, do: 1, else: 0)::8>>
  end

  def unpack(data) do
    {sku, data} = Wire.take_str(data)
    {product_name, <<quantity::little-16, rest::binary>>} = Wire.take_str(data)
    {unit_price, <<discount_pct::8, gift_wrap::8, rest::binary>>} = Money.unpack(rest)

    {%__MODULE__{
       sku: sku,
       product_name: product_name,
       quantity: quantity,
       unit_price: unit_price,
       discount_pct: discount_pct,
       gift_wrap: gift_wrap != 0
     }, rest}
  end
end

defmodule OrderRecord do
  @moduledoc """
  Mirrors examples/cases/order.yaml — a placed order.

  Between them the fields cover the four composite types nothing else in the
  suite exercises end to end: an `enum`, a `list<struct>`, a
  `map<string,struct>` and an `optional<struct>`. `billing_address` is the
  suite's only optional<struct>, so this is the first exercise of the binding's
  `{:optional, :struct}` clause.

  An enum needs nothing from the binding: it travels as its variant *name*, so
  the field is a plain string. The u8 ordinal in the layout is this worker's own
  choice, which is why @statuses has to match the case file's declaration order.

  Go is the --ref language and owns the layout; see examples/go/wire.go.
  """
  use WorkerLib.Serify.Model

  alias Customer.Wire

  # Declaration order of the `status` enum in examples/cases/order.yaml.
  @statuses ["pending", "paid", "shipped", "delivered", "cancelled"]

  defstruct order_id: 0,
            customer_id: 0,
            created_at: 0,
            status: "",
            items: [],
            subtotal: %Money{},
            adjustments: %{},
            total: %Money{},
            shipping_address: %Address{},
            billing_address: nil,
            coupon_codes: [],
            tracking_number: nil

  serify_field(:order_id, :u64)
  serify_field(:customer_id, :u64)
  serify_field(:created_at, :i64)
  serify_field(:status, :string)
  serify_field(:items, {:list, :struct}, module: LineItem)
  serify_field(:subtotal, :struct, module: Money)
  serify_field(:adjustments, :map, of: {:struct, Money})
  serify_field(:total, :struct, module: Money)
  serify_field(:shipping_address, :struct, module: Address)
  serify_field(:billing_address, {:optional, :struct}, module: Address)
  serify_field(:coupon_codes, {:list, :string})
  serify_field(:tracking_number, {:optional, :string})

  def marshal(%__MODULE__{} = o) do
    # enum: a u8 ordinal, the variant's position in the case file.
    ord =
      case Enum.find_index(@statuses, &(&1 == o.status)) do
        nil -> raise ArgumentError, "unknown order status #{inspect(o.status)}"
        i -> i
      end

    # Entry order is the map's own — deliberately not sorted. A map is
    # unordered, so order declares `oracle: semantic` and the decoded value is
    # what gets compared. See docs/protocol.md.
    adjustments =
      Enum.map_join(o.adjustments, fn {k, m} -> Wire.str(k) <> Money.pack(m) end)

    # optional<struct>: a presence flag, then the struct's fields inline.
    billing =
      case o.billing_address do
        nil -> <<0>>
        a -> <<1>> <> Address.pack(a)
      end

    tracking =
      case o.tracking_number do
        nil -> <<0>>
        s -> <<1>> <> Wire.str(s)
      end

    <<o.order_id::little-64, o.customer_id::little-64, o.created_at::signed-little-64, ord::8,
      length(o.items)::little-32>> <>
      Enum.map_join(o.items, &LineItem.pack/1) <>
      Money.pack(o.subtotal) <>
      <<map_size(o.adjustments)::little-32>> <>
      adjustments <>
      Money.pack(o.total) <>
      Address.pack(o.shipping_address) <>
      billing <>
      <<length(o.coupon_codes)::little-32>> <>
      Enum.map_join(o.coupon_codes, &Wire.str/1) <> tracking
  end

  def unmarshal(data) do
    <<order_id::little-64, customer_id::little-64, created_at::signed-little-64, ord::8,
      n_items::little-32, rest::binary>> = data

    status = Enum.at(@statuses, ord) || raise ArgumentError, "status ordinal #{ord} out of range"

    {items, rest} = Wire.take_n(rest, n_items, &LineItem.unpack/1)
    {subtotal, <<n_adj::little-32, rest::binary>>} = Money.unpack(rest)

    {adj_pairs, rest} =
      Wire.take_n(rest, n_adj, fn d ->
        {k, d} = Wire.take_str(d)
        {m, d} = Money.unpack(d)
        {{k, m}, d}
      end)

    {total, rest} = Money.unpack(rest)
    {shipping_address, <<has_billing::8, rest::binary>>} = Address.unpack(rest)

    {billing_address, rest} =
      if has_billing == 0, do: {nil, rest}, else: Address.unpack(rest)

    <<n_coupons::little-32, rest::binary>> = rest

    {coupon_codes, <<has_tracking::8, rest::binary>>} =
      Wire.take_n(rest, n_coupons, &Wire.take_str/1)

    {tracking_number, _rest} =
      if has_tracking == 0, do: {nil, rest}, else: Wire.take_str(rest)

    %__MODULE__{
      order_id: order_id,
      customer_id: customer_id,
      created_at: created_at,
      status: status,
      items: items,
      subtotal: subtotal,
      adjustments: Map.new(adj_pairs),
      total: total,
      shipping_address: shipping_address,
      billing_address: billing_address,
      coupon_codes: coupon_codes,
      tracking_number: tracking_number
    }
  end
end
