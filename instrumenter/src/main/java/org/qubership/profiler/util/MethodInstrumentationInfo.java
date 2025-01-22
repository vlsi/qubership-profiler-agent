package org.qubership.profiler.util;

import org.qubership.profiler.configuration.Rule;

public class MethodInstrumentationInfo {
    public final Rule rule;
    public final int firstLineNumber;

    public MethodInstrumentationInfo(Rule rule, int firstLineNumber) {
        this.rule = rule;
        this.firstLineNumber = firstLineNumber;
    }
}
