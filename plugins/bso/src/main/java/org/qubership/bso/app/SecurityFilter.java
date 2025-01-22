package org.qubership.bso.app;

import org.qubership.profiler.agent.Profiler;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

public class SecurityFilter {
    private static volatile Method mdcPut$profiler;

    private void logBsoReqSize$profiler(String request) {
        putMDC$profiler("bso.req.size", Integer.toString(request.length()));
    }

    private void putMDC$profiler(String name, String value) {
        Method mdcPut = mdcPut$profiler;
        if (mdcPut == null) {
            try {
                Class<?> mdc = Class.forName("org.slf".toString() + "4j.MDC");
                mdcPut = mdc.getMethod("put", String.class, String.class);
            } catch (ClassNotFoundException|NoSuchMethodException e) {
                Profiler.logWarn("Unable to pass mac/inline info to MDC. You can ignore this warning", e);
            }
            mdcPut$profiler = mdcPut;
        }
        if (mdcPut != null) {
            try {
                mdcPut.invoke(null, name, value);
            } catch (IllegalAccessException|InvocationTargetException e) {
                Profiler.logWarn("Unable to pass mac/inline info to MDC. You can ignore this warning", e);
            }
        }
    }
}
