package org.qubership.profiler.io.call;

import org.qubership.profiler.dump.DataInputStreamEx;
import org.qubership.profiler.io.Call;

import java.io.IOException;
import java.util.BitSet;

public class CallDataReader_03 extends CallDataReader_02 {
    public void read(Call dst, DataInputStreamEx calls, BitSet requiredIds) throws IOException {
        super.read(dst, calls, requiredIds);
        dst.fileRead = calls.readVarLong();
        dst.fileWritten = calls.readVarLong();
        dst.netRead = calls.readVarLong();
        dst.netWritten = calls.readVarLong();
    }
}
