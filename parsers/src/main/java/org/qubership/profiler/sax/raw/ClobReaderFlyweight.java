package org.qubership.profiler.sax.raw;

import org.qubership.profiler.sax.values.ClobValue;

import java.sql.Clob;

public interface ClobReaderFlyweight extends StrReader {
    void adaptTo(ClobValue clob);
}
