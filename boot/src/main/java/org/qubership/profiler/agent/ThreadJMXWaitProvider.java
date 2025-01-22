package org.qubership.profiler.agent;

import org.qubership.profiler.agent.LocalState;

public interface ThreadJMXWaitProvider {
    public void updateThreadCounters(LocalState state);
}
