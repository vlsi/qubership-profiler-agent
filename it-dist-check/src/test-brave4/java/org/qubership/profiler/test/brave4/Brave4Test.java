package org.qubership.profiler.test.brave4;

import brave.Span;
import brave.Tracer;
import org.qubership.profiler.agent.LocalState;
import org.qubership.profiler.agent.Profiler;
import org.qubership.profiler.test.ConfigurationTest;
import org.junit.Assert;
import org.testng.annotations.Test;

public class Brave4Test extends ConfigurationTest {
    @Test
    public void brave4() {
        Profiler.enter("void org.qubership.profiler.test.Brave4Test.brave4() (Brave4Test.java:24) [unknown jar]");
        try {
            Tracer tracer = Tracer.newBuilder().traceId128Bit(true).build();
            Span span = tracer.newTrace().name("root operation").start();
            String traceId = span.context().traceIdString();
            Assert.assertNotNull("Brave.traceId is null: tracer does not work", traceId);
            span.close();

            LocalState state = Profiler.getState();
            String endToEndId = state.callInfo.getEndToEndId();
            Assert.assertEquals(
                "state.callInfo.getEndToEndId() should be equal to brave.traceId",
                endToEndId,
                traceId
            );
        } finally {
            Profiler.exit();
        }
    }
}
