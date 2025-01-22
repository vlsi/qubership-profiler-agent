package org.qubership.profiler.agent;

import org.qubership.profiler.agent.LocalState;

public interface ThreadJMXProvider {
    public void updateThreadCounters(LocalState state);
}
