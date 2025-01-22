package org.qubership.profiler.servlet.util;

import org.qubership.profiler.agent.Bootstrap;
import org.qubership.profiler.agent.DumperPlugin;
import org.qubership.profiler.agent.DumperPlugin_02;

public class DumperStatusProvider02 extends DumperStatusProvider {
    final DumperPlugin_02 dumper = (DumperPlugin_02) Bootstrap.getPlugin(DumperPlugin.class);

    public void update() {
        isStarted = dumper.isStarted();
        activeTime = System.currentTimeMillis() - dumper.getDumperStartTime();
        numberOfRestarts = dumper.getNumberOfRestarts();
        writeTime = dumper.getWriteTime();
        writtenBytes = dumper.getWrittenBytes();
        writtenRecords = dumper.getWrittenRecords();
        currentRoot = dumper.getCurrentRoot();
    }
}
