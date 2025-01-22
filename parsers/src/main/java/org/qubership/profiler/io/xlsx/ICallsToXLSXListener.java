package org.qubership.profiler.io.xlsx;

import org.qubership.profiler.io.CallListener;

public interface ICallsToXLSXListener extends CallListener {
    void postProcess();
    void processError(Throwable ex);
}
