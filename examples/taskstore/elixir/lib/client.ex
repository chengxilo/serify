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

defmodule Client do
  @moduledoc """
  The Elixir client for the taskstore server.

      mix run -e 'Client.main(System.argv())' -- list
      mix run -e 'Client.main(System.argv())' -- create "Buy milk" normal errand,home 1755820800
      mix run -e 'Client.main(System.argv())' -- read 1001
      mix run -e 'Client.main(System.argv())' -- update 1001 "Buy oat milk" true high errand
      mix run -e 'Client.main(System.argv())' -- delete 1001

  The escript's entry point is `Worker`, and mix allows one per project, so the
  client is reached with `mix run` instead of being built into a second binary.

  Same arguments and same bytes as the other clients, so it talks to the Go
  server without either side knowing which language the other is.
  """

  @usage """
  usage: Client [--addr host:port] <command> [args]

    list
    create <title> [priority] [tag,tag] [due-unix]
    read   <id>
    update <id> <title> <done> [priority] [tag,tag] [due-unix]
    delete <id>

  priority is low, normal or high (default normal).\
  """

  def main(argv) do
    {addr, args} =
      case argv do
        ["--addr", addr | rest] -> {addr, rest}
        rest -> {"127.0.0.1:9977", rest}
      end

    case parse_op(args) do
      {:error, reason} ->
        IO.puts(:stderr, "#{reason}\n\n#{@usage}")
        System.halt(2)

      {:ok, op} ->
        send_and_print(addr, %Request{request_id: 1, op: op})
    end
  end

  defp send_and_print(addr, req) do
    case send_request(addr, req) do
      {:error, reason} ->
        IO.puts(:stderr, "#{addr}: #{inspect(reason)}")
        System.halt(1)

      {:ok, %Response{result: result}} ->
        IO.puts(render(result))
        if match?({:failed, _}, result), do: System.halt(1)
    end
  end

  # Opens a connection, writes one request frame and reads one response frame.
  defp send_request(addr, req) do
    [host, port] = String.split(addr, ":", parts: 2)

    with {:ok, socket} <-
           :gen_tcp.connect(String.to_charlist(host), String.to_integer(port),
             [:binary, active: false, packet: :raw]) do
      try do
        with :ok <- Frame.write(socket, Request.marshal(req)),
             {:ok, payload} <- Frame.read(socket) do
          {:ok, Response.unmarshal(payload)}
        end
      after
        :gen_tcp.close(socket)
      end
    end
  end

  defp parse_op([]), do: {:error, "no command given"}
  defp parse_op(["list" | _]), do: {:ok, :list_all}

  defp parse_op(["create", title | rest]) do
    with {:ok, priority} <- parse_priority(arg(rest, 0, "normal")) do
      draft = %Draft{
        title: title,
        priority: priority,
        tags: parse_tags(arg(rest, 1)),
        due_at: parse_due(arg(rest, 2))
      }

      # A struct payload travels as a field map; see the note in Request.
      {:ok, {:create, Draft.to_field_map(draft)}}
    end
  end

  defp parse_op(["create"]), do: {:error, "create needs a title"}

  defp parse_op([cmd, id | _]) when cmd in ["read", "delete"] do
    with {:ok, id} <- parse_id(id) do
      {:ok, {String.to_existing_atom(cmd), id}}
    end
  end

  defp parse_op([cmd]) when cmd in ["read", "delete"], do: {:error, "#{cmd} needs an id"}

  defp parse_op(["update", id, title, done | rest]) when done in ["true", "false"] do
    with {:ok, id} <- parse_id(id),
         {:ok, priority} <- parse_priority(arg(rest, 0, "normal")) do
      task = %TaskItem{
        id: id,
        title: title,
        done: done == "true",
        priority: priority,
        tags: parse_tags(arg(rest, 1)),
        due_at: parse_due(arg(rest, 2))
      }

      {:ok, {:update, TaskItem.to_field_map(task)}}
    end
  end

  defp parse_op(["update", _, _, done | _]),
    do: {:error, "bad done #{inspect(done)}, want true or false"}

  defp parse_op(["update" | _]), do: {:error, "update needs an id, a title and done"}
  defp parse_op([cmd | _]), do: {:error, "unknown command #{inspect(cmd)}"}

  defp arg(list, i, fallback \\ "") do
    case Enum.at(list, i) do
      nil -> fallback
      "" -> fallback
      v -> v
    end
  end

  defp parse_id(raw) do
    case Integer.parse(raw) do
      {id, ""} when id >= 0 -> {:ok, id}
      _ -> {:error, "bad id #{inspect(raw)}"}
    end
  end

  # Rejects a bad priority here, before anything is encoded. This is the only
  # place in the project where one can exist: an enum has no wire representation
  # outside its declared variants, so by the time a request is bytes the value is
  # already known to be good — which is why the server does not check it again.
  defp parse_priority(s) do
    if s in Wire.priorities() do
      {:ok, s}
    else
      {:error, "#{inspect(s)} is not a priority (#{Enum.join(Wire.priorities(), ", ")})"}
    end
  end

  defp parse_tags(""), do: []
  defp parse_tags(s), do: String.split(s, ",")

  defp parse_due(s) do
    case Integer.parse(s) do
      {due, ""} -> due
      _ -> nil
    end
  end

  defp render(:accepted), do: "accepted"
  defp render({:found, m}), do: render_task(TaskItem.from_field_map(m))

  defp render({:listing, m}) do
    page = TaskPage.from_field_map(m)

    (Enum.map(page.items, &render_task/1) ++
       ["(#{length(page.items)} shown, #{page.total} total)"])
    |> Enum.join("\n")
  end

  defp render({:failed, m}) do
    err = ApiError.from_field_map(m)
    "error: #{err.code}: #{err.message}"
  end

  defp render_task(%TaskItem{} = t) do
    line =
      "[#{if t.done, do: "x", else: " "}] #{t.id}  " <>
        String.pad_trailing(t.title, 30) <> "  #{t.priority}"

    line = if t.tags == [], do: line, else: line <> "  #" <> Enum.join(t.tags, " #")
    if t.due_at == nil, do: line, else: line <> "  due=#{t.due_at}"
  end
end
