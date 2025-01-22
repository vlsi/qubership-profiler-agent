package org.qubership.profiler.io;

import org.qubership.profiler.agent.InflightCall_02;

import java.io.File;

public class ParamReaderFromMemory_03 extends ParamReaderFromMemory_02 {
    public ParamReaderFromMemory_03(File root) {
        super(root);
        org.qubership.profiler.agent.InflightCall_02.class.getName();
    }

    @Override
    protected Call convert(org.qubership.profiler.agent.InflightCall call) {
        Call dst = super.convert(call);
        org.qubership.profiler.agent.InflightCall_02 src = (InflightCall_02) call;
        dst.transactions = src.transactions();
        dst.queueWaitDuration = src.queueWaitDuration();
        return dst;
    }
}
