package org.qubership.profiler.io.call;

import org.qubership.profiler.dump.DataInputStreamEx;
import org.qubership.profiler.io.Call;

import java.io.IOException;

public interface ReactorCallReader {
     void read(Call dst, DataInputStreamEx calls) throws IOException;
}
