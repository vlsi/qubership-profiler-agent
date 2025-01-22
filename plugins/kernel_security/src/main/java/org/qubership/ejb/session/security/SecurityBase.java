package org.qubership.ejb.session.security;

import java.security.Principal;

public class SecurityBase {
    public native Principal getPrincipal();
}
