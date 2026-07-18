/*
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.List;

import io.serify.WorkerLib;

/**
 * Mirrors examples/cases/signals.yaml, which uses every scalar the schema allows
 * as a list element.
 *
 * <p>Java has no unsigned primitives, so a uint8 rides in a {@code Byte} and a
 * uint64 in a {@code Long} — at their maxima both read as -1, and the library
 * encodes them through the unsigned view. uint128/int128 have no primitive at
 * all and are {@code BigInteger}, as in LedgerEntry.
 *
 * <p>Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 * Each list is a u32 element count followed by its elements, little-endian.
 */
@WorkerLib.SerifyModel
public final class SignalCapture {

    // Declaration order of the `mode` enum; the index is the wire ordinal.
    private static final String[] MODES = {"idle", "active", "fault", "calibrating"};

    @WorkerLib.SerifyField("capture_id") public long captureId;
    @WorkerLib.SerifyField public List<Boolean> flags = List.of();
    @WorkerLib.SerifyField("raw_frame") public List<Byte> rawFrame = List.of();
    @WorkerLib.SerifyField("port_numbers") public List<Short> portNumbers = List.of();
    @WorkerLib.SerifyField("sample_counts") public List<Integer> sampleCounts = List.of();
    @WorkerLib.SerifyField("byte_totals") public List<Long> byteTotals = List.of();
    @WorkerLib.SerifyField("trim_offsets") public List<Byte> trimOffsets = List.of();
    @WorkerLib.SerifyField("drift_deltas") public List<Short> driftDeltas = List.of();
    @WorkerLib.SerifyField("temperatures_c") public List<Integer> temperaturesC = List.of();
    @WorkerLib.SerifyField("timestamps_ns") public List<Long> timestampsNs = List.of();
    @WorkerLib.SerifyField public List<BigInteger> counters = List.of();
    @WorkerLib.SerifyField public List<BigInteger> balances = List.of();
    @WorkerLib.SerifyField public List<Float> gains = List.of();
    @WorkerLib.SerifyField public List<Double> voltages = List.of();
    @WorkerLib.SerifyField("channel_names") public List<String> channelNames = List.of();
    @WorkerLib.SerifyField public List<byte[]> payloads = List.of();
    @WorkerLib.SerifyField public List<Byte> checksum = List.of();
    @WorkerLib.SerifyField public List<Short> window = List.of();
    @WorkerLib.SerifyField("dropped_frames") public Integer droppedFrames;
    @WorkerLib.SerifyField public String mode = "";

    private static final BigInteger TWO_128 = BigInteger.ONE.shiftLeft(128);

    /** Writes a list's u32 element count. Every list carries one, even when empty. */
    private static void writeCount(ByteArrayOutputStream out, int n) {
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(n).array());
    }

    /** 16 bytes little-endian two's complement, matching the Go worker. */
    private static void writeInt128(ByteArrayOutputStream out, BigInteger v) {
        BigInteger u = v.signum() < 0 ? v.add(TWO_128) : v;
        byte[] le = new byte[16];
        byte[] be = u.toByteArray(); // big-endian, may carry a leading sign byte
        for (int i = 0; i < be.length && i < 16; i++) {
            le[i] = be[be.length - 1 - i];
        }
        out.writeBytes(le);
    }

    private static BigInteger readInt128(ByteBuffer buf, boolean signed) {
        byte[] le = new byte[16];
        buf.get(le);
        byte[] be = new byte[16];
        for (int i = 0; i < 16; i++) {
            be[15 - i] = le[i];
        }
        BigInteger n = new BigInteger(1, be);
        return signed && n.testBit(127) ? n.subtract(TWO_128) : n;
    }

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(captureId).array());

        writeCount(out, flags.size());
        for (Boolean v : flags) out.write(v ? 1 : 0);

        writeCount(out, rawFrame.size());
        for (Byte v : rawFrame) out.write(v);

        writeCount(out, portNumbers.size());
        for (Short v : portNumbers) out.writeBytes(le(2).putShort(v).array());

        writeCount(out, sampleCounts.size());
        for (Integer v : sampleCounts) out.writeBytes(le(4).putInt(v).array());

        writeCount(out, byteTotals.size());
        for (Long v : byteTotals) out.writeBytes(le(8).putLong(v).array());

        writeCount(out, trimOffsets.size());
        for (Byte v : trimOffsets) out.write(v);

        writeCount(out, driftDeltas.size());
        for (Short v : driftDeltas) out.writeBytes(le(2).putShort(v).array());

        writeCount(out, temperaturesC.size());
        for (Integer v : temperaturesC) out.writeBytes(le(4).putInt(v).array());

        writeCount(out, timestampsNs.size());
        for (Long v : timestampsNs) out.writeBytes(le(8).putLong(v).array());

        writeCount(out, counters.size());
        for (BigInteger v : counters) writeInt128(out, v);

        writeCount(out, balances.size());
        for (BigInteger v : balances) writeInt128(out, v);

        writeCount(out, gains.size());
        for (Float v : gains) out.writeBytes(le(4).putFloat(v).array());

        writeCount(out, voltages.size());
        for (Double v : voltages) out.writeBytes(le(8).putDouble(v).array());

        writeCount(out, channelNames.size());
        for (String v : channelNames) Wire.writeLenPrefixed(out, v);

        writeCount(out, payloads.size());
        for (byte[] v : payloads) Wire.writeLenPrefixed(out, v);

        // array<T,N> carries no count: N is fixed by the schema.
        for (Byte v : checksum) out.write(v);
        for (Short v : window) out.writeBytes(le(2).putShort(v).array());

        // optional<uint32>: a presence flag, then the value if present.
        if (droppedFrames == null) {
            out.write(0);
        } else {
            out.write(1);
            out.writeBytes(le(4).putInt(droppedFrames).array());
        }

        // enum: a u8 ordinal, the variant's position in the case file.
        out.write(java.util.Arrays.asList(MODES).indexOf(mode));

        return out.toByteArray();
    }

    private static ByteBuffer le(int n) {
        return ByteBuffer.allocate(n).order(ByteOrder.LITTLE_ENDIAN);
    }

    public static SignalCapture unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var s = new SignalCapture();
        s.captureId = buf.getLong();

        s.flags = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.flags.add(buf.get() != 0);

        s.rawFrame = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.rawFrame.add(buf.get());

        s.portNumbers = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.portNumbers.add(buf.getShort());

        s.sampleCounts = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.sampleCounts.add(buf.getInt());

        s.byteTotals = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.byteTotals.add(buf.getLong());

        s.trimOffsets = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.trimOffsets.add(buf.get());

        s.driftDeltas = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.driftDeltas.add(buf.getShort());

        s.temperaturesC = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.temperaturesC.add(buf.getInt());

        s.timestampsNs = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.timestampsNs.add(buf.getLong());

        s.counters = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.counters.add(readInt128(buf, false));

        s.balances = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.balances.add(readInt128(buf, true));

        s.gains = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.gains.add(buf.getFloat());

        s.voltages = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.voltages.add(buf.getDouble());

        s.channelNames = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.channelNames.add(Wire.readLenString(buf));

        s.payloads = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) s.payloads.add(Wire.readLenPrefixed(buf));

        s.checksum = new ArrayList<>();
        for (int i = 0; i < 4; i++) s.checksum.add(buf.get());
        s.window = new ArrayList<>();
        for (int i = 0; i < 3; i++) s.window.add(buf.getShort());

        s.droppedFrames = buf.get() == 0 ? null : buf.getInt();

        s.mode = MODES[buf.get() & 0xFF];

        return s;
    }
}
