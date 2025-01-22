package org.qubership.profiler.io.call;

import org.qubership.profiler.dump.DataInputStreamEx;
import org.qubership.profiler.io.Call;

import java.io.IOException;
import java.util.ArrayList;
import java.util.BitSet;
import java.util.List;

public interface CallDataReader {
    public void read(Call dst, DataInputStreamEx calls, BitSet requiredIds) throws IOException;

    public void readParams(Call dst, DataInputStreamEx calls, BitSet requiredIds) throws IOException;

    public void skipParams(Call dst, DataInputStreamEx calls) throws IOException;

    public void postCompute(ArrayList<Call> result, List<String> tags, BitSet requredIds);
}
