package org.qubership.profiler.servlet.util;

import org.qubership.profiler.agent.Bootstrap;
import org.qubership.profiler.agent.DumperPlugin;
import org.qubership.profiler.agent.DumperPlugin_04;

public class DumperStatusProvider04 extends DumperStatusProvider02 {
    DumperPlugin_04 dumper = (DumperPlugin_04) Bootstrap.getPlugin(DumperPlugin.class);

    @Override
    public void update() {
        super.update();
        bytesAllocated = dumper.getBytesAllocated();
        cpuTime = dumper.getCPUTime();
    }
}
