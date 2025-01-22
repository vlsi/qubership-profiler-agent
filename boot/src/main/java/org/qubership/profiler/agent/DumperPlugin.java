package org.qubership.profiler.agent;

import java.util.ArrayList;
import java.util.concurrent.BlockingQueue;

public interface DumperPlugin {
    public void newDumper(BlockingQueue<LocalBuffer> dirtyBuffers, BlockingQueue<LocalBuffer> emptyBuffers, ArrayList<LocalBuffer> buffers);
}
