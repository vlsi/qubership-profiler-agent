package org.qubership.profiler.agent;

public interface ThreadJMXMemoryProvider {
    public void updateThreadCounters(LocalState state);
}
