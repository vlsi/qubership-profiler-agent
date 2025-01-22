package org.qubership.bso.app;

import org.qubership.profiler.agent.Profiler;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

/**
 * Created with IntelliJ IDEA.
 * User: sitnikov
 * Date: 29/08/14
 * Time: 17:46
 * To change this template use File | Settings | File Templates.
 */
public class ICOMSClient {
    private static volatile Method mdcPut$profiler;

    private void clearMacInl$profiler() {
        putMDC$profiler("icoms.macinline", "");
        putMDC$profiler("icoms.req.size", "");
        putMDC$profiler("icoms.resp.size", "");
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
