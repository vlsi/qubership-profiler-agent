package org.qubership.profiler.agent;

public interface ThreadJMXCpuProvider {
    public void updateThreadCounters(LocalState state);
}
