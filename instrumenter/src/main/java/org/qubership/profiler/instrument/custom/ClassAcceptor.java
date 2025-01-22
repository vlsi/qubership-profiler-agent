package org.qubership.profiler.instrument.custom;

import org.qubership.profiler.instrument.ProfileClassAdapter;

public interface ClassAcceptor {
    public void onClass(ProfileClassAdapter ca, String className);
}
