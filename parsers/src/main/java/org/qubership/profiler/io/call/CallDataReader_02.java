package org.qubership.profiler.io.call;

import org.qubership.profiler.dump.DataInputStreamEx;
import org.qubership.profiler.io.Call;

import java.io.IOException;
import java.util.BitSet;

public class CallDataReader_02 extends CallDataReader_01 {
    public void read(Call dst, DataInputStreamEx calls, BitSet requiredIds) throws IOException {
        super.read(dst, calls, requiredIds);
        dst.cpuTime = calls.readVarLong();
        dst.waitTime = calls.readVarLong();
        dst.memoryUsed = calls.readVarLong();
    }
}
