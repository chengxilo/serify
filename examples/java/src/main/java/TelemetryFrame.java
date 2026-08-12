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
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import io.serify.WorkerLib;

/**
 * Mirrors examples/cases/telemetry.yaml — one reading from a field device.
 *
 * <p>This is the type that covers the corners the other examples do not: a
 * {@code uint128} address, two differently shaped fixed arrays, the suite's only
 * {@code optional<scalar>}, a {@code map<string,uint64>}, and float cases running
 * through NaN, ±Inf and negative zero. Only {@code binary} is declared, because
 * NaN and Inf have no JSON spelling.
 *
 * <p>Java has no unsigned primitives, so a uint16 rides in a {@code short} and a
 * uint64 in a {@code long} — at their maxima both read as -1, and the library
 * encodes them through the unsigned view. uint128 has no primitive at all and is
 * a {@code BigInteger}. The one place the declared type carries real meaning is
 * {@code Float humidityPct}: the boxed type is how {@code optional<float32>} says
 * "absent", which a primitive could not.
 *
 * <p>Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */
@WorkerLib.SerifyModel
public final class TelemetryFrame {

    @WorkerLib.SerifyField("device_id") public long deviceId;
    @WorkerLib.SerifyField public BigInteger ipv6 = BigInteger.ZERO;
    @WorkerLib.SerifyField("local_ip") public List<Byte> localIp = List.of();
    @WorkerLib.SerifyField public String firmware = "";
    @WorkerLib.SerifyField("boot_count") public short bootCount;
    @WorkerLib.SerifyField("rssi_dbm") public byte rssiDbm;
    @WorkerLib.SerifyField("temperature_dc") public short temperatureDc;
    @WorkerLib.SerifyField("clock_drift_ms") public int clockDriftMs;
    @WorkerLib.SerifyField("battery_volts") public float batteryVolts;
    @WorkerLib.SerifyField public double latitude;
    @WorkerLib.SerifyField public double longitude;
    @WorkerLib.SerifyField("humidity_pct") public Float humidityPct;
    @WorkerLib.SerifyField("accel_mg") public List<Short> accelMg = List.of();
    @WorkerLib.SerifyField("visible_cells") public List<Integer> visibleCells = List.of();
    @WorkerLib.SerifyField("packet_counts") public Map<String, Long> packetCounts = Map.of();
    @WorkerLib.SerifyField("gps_fix") public boolean gpsFix;
    @WorkerLib.SerifyField public byte[] signature = new byte[0];

    /** 16 bytes little-endian. uint128 is unsigned, so there is no sign to re-apply. */
    private static void writeUint128(ByteArrayOutputStream out, BigInteger v) {
        byte[] le = new byte[16];
        byte[] be = v.toByteArray(); // big-endian, may carry a leading sign byte
        for (int i = 0; i < be.length && i < 16; i++) {
            le[i] = be[be.length - 1 - i];
        }
        out.writeBytes(le);
    }

    private static BigInteger readUint128(ByteBuffer buf) {
        byte[] le = new byte[16];
        buf.get(le);
        byte[] be = new byte[16];
        for (int i = 0; i < 16; i++) {
            be[15 - i] = le[i];
        }
        return new BigInteger(1, be); // the 1 is the sign: always positive
    }

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();

        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(deviceId).array());
        writeUint128(out, ipv6);

        // array<T,N> carries no count: N is fixed by the schema.
        for (Byte v : localIp) out.write(v);

        Wire.writeLenPrefixed(out, firmware);

        out.writeBytes(ByteBuffer.allocate(2).order(ByteOrder.LITTLE_ENDIAN).putShort(bootCount).array());
        out.write(rssiDbm);
        out.writeBytes(ByteBuffer.allocate(2).order(ByteOrder.LITTLE_ENDIAN).putShort(temperatureDc).array());
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(clockDriftMs).array());
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putFloat(batteryVolts).array());
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putDouble(latitude).array());
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putDouble(longitude).array());

        // optional<float32>: a presence flag, then the value if present.
        if (humidityPct == null) {
            out.write(0);
        } else {
            out.write(1);
            out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putFloat(humidityPct).array());
        }

        for (Short v : accelMg) {
            out.writeBytes(ByteBuffer.allocate(2).order(ByteOrder.LITTLE_ENDIAN).putShort(v).array());
        }

        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(visibleCells.size()).array());
        for (Integer v : visibleCells) {
            out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(v).array());
        }

        // Entry order is the map's own — deliberately not sorted. A map is
        // unordered, so telemetry declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(packetCounts.size()).array());
        for (Map.Entry<String, Long> e : packetCounts.entrySet()) {
            Wire.writeLenPrefixed(out, e.getKey());
            out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(e.getValue()).array());
        }

        out.write(gpsFix ? 1 : 0);
        Wire.writeLenPrefixed(out, signature);

        return out.toByteArray();
    }

    public static TelemetryFrame unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var t = new TelemetryFrame();

        t.deviceId = buf.getLong();
        t.ipv6 = readUint128(buf);

        t.localIp = new ArrayList<>();
        for (int i = 0; i < 4; i++) t.localIp.add(buf.get());

        t.firmware = Wire.readLenString(buf);

        t.bootCount = buf.getShort();
        t.rssiDbm = buf.get();
        t.temperatureDc = buf.getShort();
        t.clockDriftMs = buf.getInt();
        t.batteryVolts = buf.getFloat();
        t.latitude = buf.getDouble();
        t.longitude = buf.getDouble();

        t.humidityPct = buf.get() == 0 ? null : buf.getFloat();

        t.accelMg = new ArrayList<>();
        for (int i = 0; i < 3; i++) t.accelMg.add(buf.getShort());

        t.visibleCells = new ArrayList<>();
        for (int i = buf.getInt(); i > 0; i--) t.visibleCells.add(buf.getInt());

        t.packetCounts = new LinkedHashMap<>();
        for (int i = buf.getInt(); i > 0; i--) {
            String k = Wire.readLenString(buf);
            t.packetCounts.put(k, buf.getLong());
        }

        t.gpsFix = buf.get() != 0;
        t.signature = Wire.readLenPrefixed(buf);

        return t;
    }
}
