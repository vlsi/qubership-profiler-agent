package org.qubership.profiler.io;

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.io.OutputStream;

public interface IDumpExporter {
    void exportDump(HttpServletRequest req, HttpServletResponse resp) throws IOException;
    void exportGC(String podName, String streamName, OutputStream out, TemporalRequestParams temporal) throws IOException;
}
