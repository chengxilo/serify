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

defmodule Taskstore.MixProject do
  use Mix.Project

  def project do
    [
      app: :taskstore,
      version: "0.1.0",
      elixir: "~> 1.17",
      # name: the escript is called `worker` rather than `taskstore` so that
      # worker.yaml's run command matches every other example — and so the
      # repo's .gitignore, which ignores **/worker, keeps the build out of git.
      escript: [main_module: Worker, name: "worker"],
      deps: deps()
    ]
  end

  def application do
    [extra_applications: [:logger]]
  end

  defp deps do
    [{:serify, path: "../../../lib/elixir"}]
  end
end
