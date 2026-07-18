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

"""Python example worker.

This file is the whole worker: it puts the serify library on the import path and
names the types it can handle, one serializer/deserializer pair per format.
Everything else lives in the modules beside it — those stand in for the types an
application already owns, each carrying its own schema binding and byte layout.

Types this worker does not register (customer, order, telemetry) are reported to
the runner as SKIPPED.
"""

import os
import sys

_lib_dir = os.path.join(os.path.dirname(__file__), '../../lib/python')
if not os.path.isdir(_lib_dir):
    raise RuntimeError(f"serify library not found at {_lib_dir}; fix the relative path")
sys.path.insert(0, _lib_dir)

from ledger import LedgerEntry  # noqa: E402  (imports serify, so it follows the path setup)
from notification import NotificationRecord  # noqa: E402
from serify import run_suite  # noqa: E402
from signals import SignalCapture  # noqa: E402


def _binary(model):
    """One (serializer, deserializer) pair for a model that carries its own
    byte layout: FieldMap in, model, bytes out — and back."""
    return (
        lambda fm: model.from_field_map(fm).marshal(),
        lambda data: model.unmarshal(data).to_field_map(),
    )


if __name__ == '__main__':
    run_suite({
        "ledger": {"binary": _binary(LedgerEntry)},
        "signals": {"binary": _binary(SignalCapture)},
        "notification": {"binary": _binary(NotificationRecord)},
    })
