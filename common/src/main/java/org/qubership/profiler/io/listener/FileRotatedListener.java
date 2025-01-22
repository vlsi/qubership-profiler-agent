package org.qubership.profiler.io.listener;

import org.qubership.profiler.dump.DumpFile;

import java.io.File;

/**
 * @author logunov
 */
public interface FileRotatedListener {
    void fileRotated(DumpFile oldFile, DumpFile newFile);
}
