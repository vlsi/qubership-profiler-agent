package org.qubership.profiler.agent;

public interface InflightCall_01 extends InflightCall {
    public long fileRead();

    public long fileWritten();

    public long netRead();

    public long netWritten();
}
