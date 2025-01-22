package org.qubership.profiler.configuration.callfilters;

import org.qubership.profiler.agent.FilterOperator;

import java.util.Map;

public class FilterOperatorAnd extends FilterOperatorLogical {

    public boolean evaluate(Map<String, Object> params) {
        for (FilterOperator ancestor : children) {
            if (!ancestor.evaluate(params)) {
                return false;
            }
        }

        return true;
    }
}
