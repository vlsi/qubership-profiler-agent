package org.qubership.profiler.servlet.util;

import org.qubership.profiler.agent.Bootstrap;
import org.qubership.profiler.agent.DumperPlugin;
import org.qubership.profiler.agent.DumperPlugin_06;

public class DumperStatusProvider06 extends DumperStatusProvider05 {
    DumperPlugin_06 dumper = (DumperPlugin_06) Bootstrap.getPlugin(DumperPlugin.class);

    @Override
    public void update() {
        super.update();
        uncompressedSize = dumper.getUncompressedSize();
    }
}
