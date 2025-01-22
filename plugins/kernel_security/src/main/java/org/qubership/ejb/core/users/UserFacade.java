package org.qubership.ejb.core.users;

public class UserFacade {
    public static native UserFacade getInstance();
    public native String getCurrentUserName();
}
