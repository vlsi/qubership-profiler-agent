package org.qubership.bso.app;

import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.agent.StringUtils;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

public class AbstractDBtoXMLModule {
    protected String procedureName;
    protected String sqlText;
    protected String[] procedureParameters;
    protected String[] sqlParams;

    private static volatile Method mdcPut$profiler;

    private void dumpSql$profiler() {
        if (sqlText == null) {
            Profiler.event(procedureName, "sql");
            Profiler.event(StringUtils.arrayToString(new StringBuilder(), procedureParameters).toString(), "binds");
        } else {
            Profiler.event(sqlText, "sql");
            Profiler.event(StringUtils.arrayToString(new StringBuilder(), sqlParams).toString(), "binds");
        }
        String dbcall = sqlText == null ? "call " + procedureName : "sql " + sqlText.replaceAll("[\r\n]", " ");
        putMDC$profiler("icoms.dbcall", dbcall);
    }

    private void clearSql$profiler() {
        putMDC$profiler("icoms.dbcall", "");
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
