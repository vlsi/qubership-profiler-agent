package org.qubership.profiler.agent.plugins;

import org.qubership.profiler.agent.Configuration_06;
import org.qubership.profiler.agent.DefaultMethodImplInfo;
import org.qubership.profiler.configuration.Rule;

import java.util.Collection;
import java.util.List;

public interface ConfigurationSPI extends Configuration_06 {
    public Collection<Rule> getRulesForClass(String className, Collection<Rule> rules);

    /**
     * Returns the path of the configuration file was used to load the configuration
     *
     * @return path to the configuration file (relative or absolute)
     */
    public String getConfigFile();

    public List<DefaultMethodImplInfo> getDefaultMethods(String className);
}
