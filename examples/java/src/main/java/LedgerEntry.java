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

import io.serify.WorkerLib;
import io.serify.WorkerLib.SerifyField;

import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;

/**
 * Mirrors examples/cases/ledger.yaml.
 *
 * <p>The annotations are the entire schema binding — nothing here calls a
 * FieldMap accessor. Everything else is the byte layout, which is the part a
 * conformance worker exists to exercise.
 *
 * <p>Java has no 128-bit primitive type, so the two int128 fields are
 * BigInteger and converted to 16 little-endian two's-complement bytes by hand —
 * ByteBuffer has no 128-bit accessor.
 *
 * <p>Go is the --ref language and owns the layout; see examples/go/wire.go.
 */
@WorkerLib.SerifyModel
public final class LedgerEntry {

    @SerifyField public Long entryId = 0L;
    @SerifyField public Long blockNumber = 0L;
    @SerifyField public Long blockTime = 0L;
    @SerifyField public byte[] txHash = new byte[0];
    @SerifyField public String account = "";
    @SerifyField public String asset = "";
    @SerifyField public BigInteger amountBaseUnits = BigInteger.ZERO;
    @SerifyField public BigInteger balanceAfter = BigInteger.ZERO;
    @SerifyField public Boolean confirmed = false;
    @SerifyField public String memo = null;

    private static final BigInteger TWO_128 = BigInteger.ONE.shiftLeft(128);
    private static final BigInteger MASK_128 = TWO_128.subtract(BigInteger.ONE);
    private static final int INT128_BYTES = 16;

    /**
     * int128 as 16 little-endian two's-complement bytes. Masking to 128 bits maps a
     * negative onto its residue class, which is exactly two's complement; toByteArray
     * then gives big-endian bytes that only need reversing.
     */
    private static byte[] writeInt128LE(BigInteger v) {
        byte[] be = v.and(MASK_128).toByteArray();
        byte[] le = new byte[INT128_BYTES];
        for (int i = 0; i < INT128_BYTES && i < be.length; i++) {
            le[i] = be[be.length - 1 - i];
        }
        return le;
    }

    private static BigInteger readInt128LE(ByteBuffer buf) {
        byte[] le = new byte[INT128_BYTES];
        buf.get(le);
        byte[] be = new byte[INT128_BYTES];
        for (int i = 0; i < INT128_BYTES; i++) {
            be[INT128_BYTES - 1 - i] = le[i];
        }
        BigInteger n = new BigInteger(1, be);
        return n.testBit(127) ? n.subtract(TWO_128) : n; // re-apply the sign
    }

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();

        out.writeBytes(ByteBuffer.allocate(24).order(ByteOrder.LITTLE_ENDIAN)
                .putLong(entryId).putLong(blockNumber).putLong(blockTime).array());

        Wire.writeLenPrefixed(out, txHash);
        Wire.writeLenPrefixed(out, account);
        Wire.writeLenPrefixed(out, asset);

        out.writeBytes(writeInt128LE(amountBaseUnits));
        out.writeBytes(writeInt128LE(balanceAfter));

        out.write(confirmed ? 1 : 0);
        if (memo == null) {
            out.write(0);
        } else {
            out.write(1);
            Wire.writeLenPrefixed(out, memo);
        }
        return out.toByteArray();
    }

    public static LedgerEntry unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var e = new LedgerEntry();

        e.entryId = buf.getLong();
        e.blockNumber = buf.getLong();
        e.blockTime = buf.getLong();

        e.txHash = Wire.readLenPrefixed(buf);
        e.account = Wire.readLenString(buf);
        e.asset = Wire.readLenString(buf);

        e.amountBaseUnits = readInt128LE(buf);
        e.balanceAfter = readInt128LE(buf);

        e.confirmed = buf.get() != 0;
        boolean hasMemo = buf.get() != 0;
        e.memo = hasMemo ? Wire.readLenString(buf) : null;

        return e;
    }
}
