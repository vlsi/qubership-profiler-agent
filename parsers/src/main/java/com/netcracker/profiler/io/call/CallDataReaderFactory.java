package com.netcracker.profiler.io.call;

import java.util.Map;

import jakarta.inject.Inject;
import jakarta.inject.Singleton;

/**
 * Factory for creating CallDataReader instances based on file format version.
 * Uses Guice MapBinder to manage the mapping of format versions to implementations.
 */
@Singleton
public class CallDataReaderFactory {
    private final Map<Integer, CallDataReader> readers;
    private final CallDataReader defaultReader;

    @Inject
    public CallDataReaderFactory(Map<Integer, CallDataReader> readers) {
        this.readers = readers;
        // Default reader is format 0
        this.defaultReader = readers.getOrDefault(0, new CallDataReader_00());
    }

    /**
     * Creates a CallDataReader for the specified file format.
     * @param fileFormat The file format version (0-4)
     * @return CallDataReader instance for the specified format
     */
    public CallDataReader createReader(int fileFormat) {
        return readers.getOrDefault(fileFormat, defaultReader);
    }
}
