package org.qubership.bso.app;

import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.agent.StringUtils;

import javax.servlet.ServletOutputStream;
import java.io.ByteArrayOutputStream;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

public class CheckErrorResponseWrapper {
    private static volatile Method mdcPut$profiler;

    private class CheckOutputStream {
        public native ByteArrayOutputStream getStream();
    }

    public native ServletOutputStream getOutputStream();

    private void logBsoRespSize$profiler() {
        try {
            ServletOutputStream outputStream = getOutputStream();
            CheckOutputStream stream = (CheckOutputStream) outputStream;
            ByteArrayOutputStream baos = stream.getStream();
            putMDC$profiler("bso.resp.size", Integer.toString(baos.size()));
        } catch (Throwable t) {
            Profiler.event(StringUtils.throwableToString(t), "exception");
        }
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
