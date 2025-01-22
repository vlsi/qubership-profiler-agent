package org.qubership.profiler.io;

import org.qubership.profiler.configuration.ParameterInfoDto;

import java.util.List;

public interface IParamsStreamVisitor {

    void visitParam(String name, boolean index, boolean list, int order, String signature);

    List<ParameterInfoDto> getAndCleanParams();
}
