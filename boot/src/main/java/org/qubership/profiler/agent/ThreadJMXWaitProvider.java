package org.qubership.profiler.agent;

public interface ThreadJMXWaitProvider {
    public void updateThreadCounters(LocalState state);
}
