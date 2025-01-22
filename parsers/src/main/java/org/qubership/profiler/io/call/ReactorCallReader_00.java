package org.qubership.profiler.io.call;

import org.qubership.profiler.dump.DataInputStreamEx;
import org.qubership.profiler.io.Call;

import java.io.IOException;

public class ReactorCallReader_00 implements ReactorCallReader {

    @Override
    public void read(Call dst, DataInputStreamEx calls) throws IOException {
        dst.nonBlocking = calls.readVarInt();
        if (calls.read() == 1) {
            dst.reactorChainId = calls.readString();
        } else {
            dst.reactorChainId = null;
        }
        //duplicate these three since we need them to search in reactor trace by reactorChainId
        dst.traceFileIndex = calls.readVarInt();
        dst.bufferOffset = calls.readVarInt();
        dst.recordIndex = calls.readVarInt();
        dst.reactorFileIndex = calls.readVarInt();
        dst.reactorBufferOffset = calls.readVarInt();
    }
}
