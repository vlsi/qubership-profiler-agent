package org.qubership.schemas.dataflow.bso.functional.messages;

import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.agent.CallInfo;

/**
 * BSO request header
 */
public class RequestHeader {
    protected String processID;

    public void addCaseName$profiler() {
        CallInfo callInfo = Profiler.getState().callInfo;
        String endToEndId = callInfo.getEndToEndId();
        if (endToEndId != null && !processID.endsWith(endToEndId)) {
            processID += "#-" + endToEndId;
            if (processID.length() > 99) {
                processID = processID.substring(0, 100);
            }
        }

        Profiler.event(processID, "bso.process.id");
    }
}
