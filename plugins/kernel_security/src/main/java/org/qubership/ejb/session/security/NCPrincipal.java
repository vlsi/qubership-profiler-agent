package org.qubership.ejb.session.security;

import java.security.Principal;

public class NCPrincipal {
    public static native String getPrincipalName(Principal p);
}
