package org.qubership.ejb.core.security;

import org.qubership.ejb.session.security.SecurityBase;

public class SecurityFacade {
    public static native SecurityFacade getInstance();
    public native SecurityBase getSecurityLocal();
}
