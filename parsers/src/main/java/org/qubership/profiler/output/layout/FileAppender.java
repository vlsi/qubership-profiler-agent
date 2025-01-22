package org.qubership.profiler.output.layout;

import java.io.IOException;
import java.io.OutputStream;

/**
 * Appends file with given name to the output stream.
 */
public interface FileAppender {
    void append(String fileName, OutputStream out) throws IOException;
}
