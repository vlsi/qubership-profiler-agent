package org.qubership.framework.domain.actions;
import org.qubership.framework.domain.DomainDataSource;
import org.qubership.framework.domain.DomainFacade;
import org.qubership.profiler.agent.Profiler;

public class DomainActionDelegator {
    protected static void logDds$profiler() {
        try {
            DomainDataSource localDDS = DomainFacade.getInstance();
            if (localDDS == null) {
                return;
            }
            Profiler.event(localDDS.getCurrentContextDataSource(), "current.domain");
        } catch (Throwable e) {
            /* ignore */
        }
    }
}
