package org.qubership.profiler.io;

import org.qubership.profiler.agent.InflightCall_01;

import java.io.File;

public class ParamReaderFromMemory_02 extends ParamReaderFromMemory_01 {
    public ParamReaderFromMemory_02(File root) {
        super(root);
        org.qubership.profiler.agent.InflightCall_01.class.getName();
    }

    @Override
    protected Call convert(org.qubership.profiler.agent.InflightCall call) {
        Call dst = super.convert(call);
        org.qubership.profiler.agent.InflightCall_01 src = (InflightCall_01) call;
        dst.fileRead = src.fileRead();
        dst.fileWritten = src.fileWritten();
        dst.netRead = src.netRead();
        dst.netWritten = src.netWritten();
        return dst;
    }
}
