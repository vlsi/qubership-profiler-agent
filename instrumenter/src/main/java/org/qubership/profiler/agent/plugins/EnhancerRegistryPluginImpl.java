package org.qubership.profiler.agent.plugins;

import org.qubership.profiler.agent.Bootstrap;
import org.qubership.profiler.agent.EnhancerRegistryPlugin;
import org.qubership.profiler.instrument.enhancement.EnhancerPlugin;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

public class EnhancerRegistryPluginImpl implements EnhancerRegistryPlugin {
    private final Map<String, EnhancerPlugin> enhancers = new HashMap<String, EnhancerPlugin>();

    public EnhancerRegistryPluginImpl() {
        Bootstrap.registerPlugin(EnhancerRegistryPlugin.class, this);
    }

    public void addEnhancerPlugin(String name, Object plugin) {
        enhancers.put(name, (EnhancerPlugin) plugin);
    }

    public Object getEnhancerPlugin(String name) {
        return enhancers.get(name);
    }

    public Map<String, Object> getEnhancersMap() {
        return Collections.unmodifiableMap((Map) enhancers);
    }
}
