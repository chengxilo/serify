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

defmodule Worker do
  @moduledoc """
  Elixir example worker.

  This module is the whole worker: it names the types it can handle, one
  serializer/deserializer pair per format. Everything else lives in the modules
  beside it — those stand in for the types an application already owns, each
  carrying its own schema binding and byte layout.

  telemetry is the one type this worker does not register, and it is not
  coming: the BEAM has no NaN and no infinity, and its float cases carry both.
  It is reported to the runner as SKIPPED and declared in
  examples/cases/expected_skips/elixir.yaml.
  """

  def main(_args) do
    WorkerLib.run_suite(%{
      "customer" => %WorkerLib.Type{
        model: CustomerRecord,
        formats: %{
          "binary" => {&CustomerRecord.marshal/1, &CustomerRecord.unmarshal/1},
          "json" => {&CustomerRecord.to_json/1, &CustomerRecord.from_json/1}
        }
      },
      "ledger" => binary(LedgerEntry),
      "order" => binary(OrderRecord),
      "signals" => binary(SignalCapture),
      "notification" => binary(NotificationRecord)
    })
  end

  # A model that carries its own byte layout, under the one format it speaks.
  # Registered as a %Type{}, so serify converts field map <-> model around
  # marshal/unmarshal and neither of them ever sees a field map.
  defp binary(model) do
    %WorkerLib.Type{
      model: model,
      formats: %{"binary" => {&model.marshal/1, &model.unmarshal/1}}
    }
  end
end
