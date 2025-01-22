package org.qubership.profiler.agent;

import org.qubership.profiler.agent.LocalState;

public interface ThreadJMXCpuProvider {
    public void updateThreadCounters(LocalState state);
}
