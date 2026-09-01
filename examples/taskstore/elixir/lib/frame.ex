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

defmodule Frame do
  @moduledoc """
  Framing: a u32 little-endian byte length, then that many bytes.

  Outside the conformance suite on purpose. serify tests the contents of a
  message; the frame header is in none of the cases, so a follower may read its
  socket however it likes as long as it agrees on what is inside.

  `:gen_tcp` can do length-prefixed framing itself with `packet: 4` — but that
  option is big-endian only, and this protocol is little-endian throughout, so
  the socket is opened `packet: :raw` and the header is read by hand.
  """

  @max_frame_len 1024 * 1024

  def max_frame_len, do: @max_frame_len

  def write(socket, payload) when byte_size(payload) <= @max_frame_len do
    :gen_tcp.send(socket, <<byte_size(payload)::little-32, payload::binary>>)
  end

  def write(_socket, payload) do
    {:error, "message of #{byte_size(payload)} bytes exceeds the #{@max_frame_len} limit"}
  end

  def read(socket) do
    with {:ok, <<n::little-32>>} <- :gen_tcp.recv(socket, 4),
         :ok <- check_len(n),
         {:ok, payload} <- recv_body(socket, n) do
      {:ok, payload}
    end
  end

  defp check_len(n) when n <= @max_frame_len, do: :ok

  defp check_len(n),
    do: {:error, "frame header claims #{n} bytes, over the #{@max_frame_len} limit"}

  # recv/2 with a byte count blocks until it has exactly that many, so there is
  # no read loop here — but it will not accept a count of zero.
  defp recv_body(_socket, 0), do: {:ok, ""}
  defp recv_body(socket, n), do: :gen_tcp.recv(socket, n)
end
