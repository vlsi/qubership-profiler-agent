package org.qubership.profiler.javaagent;

import org.qubership.profiler.agent.Bootstrap;

import java.lang.instrument.Instrumentation;


public class Agent {

    public static void premain(String agentArgs, Instrumentation inst) {
        Bootstrap.premain(agentArgs, inst);
    }

    public static void agentmain(String agentArgs, Instrumentation inst) {
        Bootstrap.premain(agentArgs, inst);
    }
}
