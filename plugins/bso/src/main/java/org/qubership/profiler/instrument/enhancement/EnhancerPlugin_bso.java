package org.qubership.profiler.instrument.enhancement;

import org.qubership.profiler.agent.Configuration_01;
import org.w3c.dom.Element;

public class EnhancerPlugin_bso extends EnhancerPlugin {
    @Override
    public void init(Element node, Configuration_01 conf) {
        super.init(node, conf);
        conf.getParameterInfo("web.url").index(true);
        conf.getParameterInfo("end.to.end.id").big(false).index(true).list(true);
    }
}
