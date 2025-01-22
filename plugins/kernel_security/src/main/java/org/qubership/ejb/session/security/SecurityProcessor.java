package org.qubership.ejb.session.security;

import org.qubership.profiler.agent.CallInfo;
import org.qubership.profiler.agent.Profiler;

import javax.security.auth.Subject;

public class SecurityProcessor {
    protected final static String INTERNAL_PRINCIPAL_NAME = "internal";

    static void logUserId$profiler() {
        logUserId$profiler(INTERNAL_PRINCIPAL_NAME);
    }

    static void logUserId$profiler(Subject subject) {
        if(subject == null || subject.getPrincipals() == null || subject.getPrincipals().isEmpty()) {
            logUserId$profiler("null");
            return;
        }
        logUserId$profiler(String.valueOf(subject.getPrincipals().iterator().next()));

    }
    static void logUserId$profiler(String userName) {
        Profiler.event(userName, "nc_user");
        saveUserId$profiler(userName);
    }

    static void saveUserId$profiler(String userName) {
        final CallInfo callInfo = Profiler.getState().callInfo;
        callInfo.setNcUser(userName);
        callInfo.setCliendId(userName + "@" + callInfo.getRemoteAddress());
    }

    static String getCurrentUserId$profiler() {
        return Profiler.getState().callInfo.getNcUser();
    }
}
