package org.qubership.profiler.test;

import static org.testng.Assert.assertNotNull;

import org.qubership.profiler.agent.Bootstrap;
import org.qubership.profiler.agent.Configuration;

import org.testng.annotations.BeforeSuite;

import java.io.IOException;

public class ConfigurationTest {
    @BeforeSuite
    public void defaultConfigurationLoads() throws IOException {
        Configuration conf = Bootstrap.getPlugin(Configuration.class);
        assertNotNull(conf, "Bootstrap.getPlugin(Configuration.class) should always be present");
    }
}
