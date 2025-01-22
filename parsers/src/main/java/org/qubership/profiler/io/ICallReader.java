package org.qubership.profiler.io;

import java.util.Collection;

public interface ICallReader {
    int SUSPEND_LOG_READER_EXTRA_TIME = Integer.getInteger("org.qubership.profiler.agent.Profiler.SUSPEND_LOG_READER_EXTRA_TIME", 60); //InMinutes

    void find();
    void setTimeFilterCondition(long begin, long end);
    Collection<Throwable> getExceptions();
}
