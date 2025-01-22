package org.qubership.platform.ccsm.bso.composer.impl;

import org.qubership.platform.ccsm.bso.composer.utils.Namespaces;
import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.agent.CallInfo;
import org.qubership.profiler.agent.LocalState;
import org.jdom.Element;

import java.util.List;

/**
 * Patch processID for CIM
 */
public class AbstractComposer {
    private static void patchProcessId$profiler(LocalState state, Element parent) {
        CallInfo callInfo = state.callInfo;
        String endToEndId = callInfo.getEndToEndId();
        if (endToEndId == null) {
            Profiler.event("endToEnd is empty", "comment");
            return;
        }
        List content = parent.getContent();
        if (content.isEmpty()){
            Profiler.event("content is empty", "comment");
            return;
        }
        Object o = content.get(content.size() - 1);
        if (!(o instanceof Element)) {
            Profiler.event("last item is not element", "comment");
            return;
        }
        Element header = (Element) o;
        Element processId = header.getChild("processID", Namespaces.HEADER_PARAMS_NAMESPACE);
        if (processId != null) {
            processId.addContent("#-" + endToEndId);
            Profiler.event(processId.getText(), "bso.process.id");
        } else {
            Profiler.event("processId is null", "comment");
        }
    }

}
