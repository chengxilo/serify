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

defmodule NotificationRecord do
  @moduledoc """
  Mirrors examples/cases/notification.yaml, whose `channel` field is a `oneof`.

  Elixir has no sum type to declare, and needs none — a tagged tuple already
  *is* a tag and a payload, so `serify_field(:channel, :oneof)` is the whole
  binding. No arm list, no converter, no registration:

      :silent                            a unit variant
      {:sms, "hi"}                       a scalar payload
      {:push, 9007199254740993}          arbitrary precision, no special type
      {:invoice, %{"currency" => ...}}   a struct payload

  Go is the --ref language and owns the byte layout; see examples/go/wire.go.
  """
  use WorkerLib.Serify.Model

  defstruct [:notification_id, :channel, :urgent]

  serify_field(:notification_id, :u32)
  serify_field(:channel, :oneof)
  serify_field(:urgent, :bool)

  def marshal(%__MODULE__{} = n) do
    # The tag ordinal is the variant's position in the case file's oneof. The
    # schema tag *names* are the binding's business, and never appear here.
    tagged =
      case n.channel do
        :silent -> <<0>>
        {:sms, s} -> <<1, byte_size(s)::little-32, s::binary>>
        {:push, v} -> <<2, v::little-64>>
        {:invoice, m} ->
          <<3, byte_size(m["currency"])::little-32, m["currency"]::binary,
            m["amount_minor"]::signed-little-64>>
      end

    urgent = if n.urgent, do: 1, else: 0
    <<n.notification_id::little-32>> <> tagged <> <<urgent::8>>
  end

  def unmarshal(data) do
    <<notification_id::little-32, ordinal::8, rest::binary>> = data

    {channel, <<urgent::8>>} =
      case ordinal do
        0 -> {:silent, rest}
        1 ->
          <<n::little-32, s::binary-size(n), tail::binary>> = rest
          {{:sms, s}, tail}
        2 ->
          <<v::little-64, tail::binary>> = rest
          {{:push, v}, tail}
        3 ->
          <<n::little-32, currency::binary-size(n), amount::signed-little-64, tail::binary>> = rest
          {{:invoice, %{"currency" => currency, "amount_minor" => amount}}, tail}

        other ->
          raise ArgumentError, "unknown channel ordinal #{other}"
      end

    %__MODULE__{notification_id: notification_id, channel: channel, urgent: urgent == 1}
  end
end
