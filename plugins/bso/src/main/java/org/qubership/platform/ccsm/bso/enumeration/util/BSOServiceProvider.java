package org.qubership.platform.ccsm.bso.enumeration.util;

import org.qubership.platform.ccsm.bso.composer.utils.HeaderParamsHelper;
import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.agent.CallInfo;

/**
 * Created with IntelliJ IDEA.
 * User: sitnikov
 * Date: 28/08/14
 * Time: 11:54
 * To change this template use File | Settings | File Templates.
 */
public class BSOServiceProvider {
    public String getProcessIdAsString$profiler() {
        CallInfo callInfo = Profiler.getState().callInfo;
        String processID = HeaderParamsHelper.getProcessID();
        String endToEndId = callInfo.getEndToEndId();
        if (endToEndId != null)
            processID += "#-" + endToEndId;
        Profiler.event(processID, "bso.process.id");
        return processID;
    }
}
