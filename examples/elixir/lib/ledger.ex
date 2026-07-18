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

defmodule LedgerEntry do
  @moduledoc """
  Mirrors examples/cases/ledger.yaml.

  The `serify_field` declarations are the entire schema binding — nothing here
  touches a raw map key. Everything else is the byte layout, which is the part a
  conformance worker exists to exercise.

  Elixir integers are arbitrary-precision and the bitstring syntax takes a
  128-bit size directly (`<<v::signed-little-128>>`), so int128 needs no special
  handling at all — the whole layout reads as one literal.

  Go is the --ref language and owns that layout; see examples/go/wire.go.
  """
  use WorkerLib.Serify.Model

  defstruct [
    :entry_id, :block_number, :block_time, :tx_hash, :account,
    :asset, :amount_base_units, :balance_after, :confirmed, :memo
  ]

  serify_field(:entry_id, :u64)
  serify_field(:block_number, :u64)
  serify_field(:block_time, :i64)
  serify_field(:tx_hash, :bytes)
  serify_field(:account, :string)
  serify_field(:asset, :string)
  serify_field(:amount_base_units, :i128)
  serify_field(:balance_after, :i128)
  serify_field(:confirmed, :bool)
  serify_field(:memo, {:optional, :string})

  def marshal(%__MODULE__{} = e) do
    confirmed = if e.confirmed, do: 1, else: 0

    memo =
      case e.memo do
        nil -> <<0>>
        s -> <<1, byte_size(s)::little-32, s::binary>>
      end

    <<
      e.entry_id::little-64,
      e.block_number::little-64,
      e.block_time::signed-little-64,
      byte_size(e.tx_hash)::little-32,
      e.tx_hash::binary,
      byte_size(e.account)::little-32,
      e.account::binary,
      byte_size(e.asset)::little-32,
      e.asset::binary,
      e.amount_base_units::signed-little-128,
      e.balance_after::signed-little-128,
      confirmed::8
    >> <> memo
  end

  def unmarshal(data) do
    <<
      entry_id::little-64,
      block_number::little-64,
      block_time::signed-little-64,
      hash_len::little-32,
      tx_hash::binary-size(hash_len),
      account_len::little-32,
      account::binary-size(account_len),
      asset_len::little-32,
      asset::binary-size(asset_len),
      amount::signed-little-128,
      balance::signed-little-128,
      confirmed::8,
      has_memo::8,
      rest::binary
    >> = data

    memo =
      if has_memo == 0 do
        nil
      else
        <<n::little-32, s::binary-size(n), _::binary>> = rest
        s
      end

    %__MODULE__{
      entry_id: entry_id,
      block_number: block_number,
      block_time: block_time,
      tx_hash: tx_hash,
      account: account,
      asset: asset,
      amount_base_units: amount,
      balance_after: balance,
      confirmed: confirmed == 1,
      memo: memo
    }
  end
end
