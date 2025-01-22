package org.qubership.profiler.agent;

import java.lang.management.ManagementFactory;
import java.lang.management.ThreadMXBean;
import java.lang.reflect.Method;

public class FindJMXImplementation {
    public static String[] find() {
        final ThreadMXBean threadMXBean = ManagementFactory.getThreadMXBean();
        String cpuMonitoringClass = "org.qubership.profiler.agent.ThreadJMXCpuEmpty";
        try {
            if (ProfilerData.THREAD_CPU && threadMXBean.isCurrentThreadCpuTimeSupported())
                cpuMonitoringClass = "org.qubership.profiler.agent.ThreadJMXCpu";
        } catch (UnsupportedOperationException e) {
            /* Ignore */
        }

        String waitMonitoringClass = "org.qubership.profiler.agent.ThreadJMXWaitEmpty";
        try {
            if (ProfilerData.THREAD_WAIT && threadMXBean.isThreadContentionMonitoringSupported()) {
                if (!threadMXBean.isThreadContentionMonitoringEnabled())
                    threadMXBean.setThreadContentionMonitoringEnabled(true);
                if (threadMXBean.isThreadContentionMonitoringEnabled())
                    waitMonitoringClass = "org.qubership.profiler.agent.ThreadJMXWait";
            }
        } catch (UnsupportedOperationException e) {
            /* Ignore */
        }

        String memoryMonitoringClass = "org.qubership.profiler.agent.ThreadJMXMemoryEmpty";
        if (ProfilerData.THREAD_MEMORY) {
            Method getThreadAllocatedBytes;
            try {
                getThreadAllocatedBytes = threadMXBean.getClass().getMethod("getThreadAllocatedBytes", long.class);
                getThreadAllocatedBytes.setAccessible(true);
                Long bytes = (Long) getThreadAllocatedBytes.invoke(threadMXBean, Thread.currentThread().getId());
                if (bytes != null)
                    memoryMonitoringClass = "org.qubership.profiler.agent.ThreadJMXMemory";
            } catch (Throwable e) {
                /* Ignore */
            }
        }

        Bootstrap.info("Profiler: using the following thread jmx monitors: " + cpuMonitoringClass + ", " + waitMonitoringClass + ", " + memoryMonitoringClass);
        return new String[]{cpuMonitoringClass, waitMonitoringClass, memoryMonitoringClass};
    }
}
