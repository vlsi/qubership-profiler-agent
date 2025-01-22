package org.qubership.profiler.io;

import java.util.List;

public interface IActivePODReporter {
    List<ActivePODReport> reportActivePODs(String searchConditionsStr, TemporalRequestParams temporalRequestParams);
}
