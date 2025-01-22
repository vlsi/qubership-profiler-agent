package org.qubership.profiler.metrics;

import org.qubership.profiler.agent.ProfilerData;

public class DirtyBuffersMetric extends AbstractSystemMetric {

    private static final MetricUnit METRIC_UNIT = MetricUnit.TOTAL;

    public DirtyBuffersMetric(String name) {
        super(name, METRIC_UNIT);
    }

    @Override
    protected String getValue() {
        return String.valueOf(ProfilerData.dirtyBuffers.size());
    }
}
