package org.qubership.profiler.servlet;

import org.qubership.profiler.dump.DumpRootResolver;
import org.qubership.profiler.fetch.FetchCallTreeFactory;
import org.qubership.profiler.io.*;
import org.qubership.profiler.io.searchconditions.BaseSearchConditions;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.Banner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.ApplicationContext;
import org.springframework.context.event.EventListener;
import org.springframework.core.io.DefaultResourceLoader;

import java.io.File;
import java.io.PrintWriter;
import java.lang.reflect.Constructor;
import java.util.Date;
import java.util.HashMap;
import java.util.Map;

import javax.servlet.ServletContextEvent;
import javax.servlet.ServletContextListener;

@SpringBootApplication(scanBasePackages = {
        "org.qubership.profiler.io",
        "org.qubership.profiler.sax.readers",
        "org.qubership.profiler.fetch",
        "org.qubership.profiler.sax.factory",
        "org.qubership.profiler.sax.builders"
})
public class SpringBootInitializer implements ServletContextListener {
    public static final String DUMP_ROOT_PROPERTY = "org.qubership.profiler.DUMP_ROOT_LOCATION";
    public static final String IS_READ_FROM_DUMP = "org.qubership.profiler.IS_READ_FROM_DUMP";
    public static final String CASSANDRA_HOST_ENV = "CASSANDRA_HOST";
    public static final String ELASTICSEARCH_HOST_ENV = "ELASTICSEARCH_HOST";
    private static final Logger log = LoggerFactory.getLogger(SpringBootInitializer.class);

    private static SpringBootInitializer MOROZOFF;

    @Autowired
    protected CallReaderFactory callReaderFactory;

    @Autowired
    protected FetchCallTreeFactory fetchCallTreeFactory;

    @Autowired
    protected ApplicationContext context;

    @Autowired
    protected ReactorChainsResolver reactorChainsResolver;

    @Autowired
    protected IDumpExporter dumpExporter;

    @Autowired
    protected LoggedContainersInfo loggedContainersInfo;

    @Autowired
    protected ExcelExporter excelExporter;

    @EventListener(ApplicationReadyEvent.class)
    public void doSomethingAfterStartup() {
        SpringBootInitializer.MOROZOFF = this;
    }

    public static CallReaderFactory callReaderFactory() {
        return MOROZOFF.callReaderFactory;
    }

    public static FetchCallTreeFactory fetchCallTreeFactory(){
        return MOROZOFF.fetchCallTreeFactory;
    }

    public static ReactorChainsResolver reactorChainsResolver(){
        return MOROZOFF.reactorChainsResolver;
    }

    public static ApplicationContext getApplicationContext(){
        return MOROZOFF.context;
    }

    public static CallToJS callToJs(PrintWriter out, CallFilterer cf){
        return MOROZOFF.context.getBean(CallToJS.class, out, cf);
    }

    public static ExcelExporter excelExporter(){
        return MOROZOFF.excelExporter;
    }

    public static String getIsReadFromDumpProperty(){
        return MOROZOFF.context.getEnvironment().getProperty("org.qubership.profiler.IS_READ_FROM_DUMP");
    }

    public static IDumpExporter dumpExporter(){
        return MOROZOFF.dumpExporter;
    }

    public static LoggedContainersInfo loggedContainersInfo() {
        return MOROZOFF.loggedContainersInfo;
    }

    public static IActivePODReporter activePODReporter(){
        return MOROZOFF.context.getBean(IActivePODReporter.class);
    }

    public static BaseSearchConditions searchConditions(String searchConditionsStr, long dateFrom, long dateTo){
        return MOROZOFF.context.getBean(BaseSearchConditions.class, searchConditionsStr, new Date(dateFrom), new Date(dateTo));
    }

    public static void init() {
        new SpringBootInitializer().contextInitialized(null);
    }
    /**
     * for some reason maven-shade-plugin replaces constructor(resourceLoader, Class[]) with constructor(resourceLoader, Object[])
     * @return
     */
    private SpringApplication constructSpringApplication(){
        for(Constructor c: SpringApplication.class.getConstructors()) {
            if(c.getParameterTypes().length == 2) {
                try {
                    return (SpringApplication)c.newInstance(new DefaultResourceLoader(this.getClass().getClassLoader()), new Class[]{SpringBootInitializer.class});
                } catch (Exception e) {
                    throw new RuntimeException(e);
                }
            }
        }
        throw new RuntimeException("Can not find a constructor with 2 arguments (resourceLoader, Class[]) ");
    }

    public void contextInitialized(ServletContextEvent servletContextEvent) {
        try{
            SpringApplication app = constructSpringApplication();

            Map<String, Object> properties = new HashMap<>();

            log.info("Initializing file storage profile");
            app.setAdditionalProfiles("filestorage");

            // disable banner
            app.setBannerMode(Banner.Mode.OFF);

            //no auto configuration of JMS, HTTP, cassandra or whatever is needed when in legacy mode
            properties.put("spring.boot.enableautoconfiguration", "false");
            properties.put(DUMP_ROOT_PROPERTY, new File(DumpRootResolver.dumpRoot).getParentFile());
            properties.put(IS_READ_FROM_DUMP, true);

            app.setDefaultProperties(properties);
            MOROZOFF = app.run().getBean(this.getClass());
        }catch (Throwable e){
            log.error("", e);
        }
        log.info("spring boot started");
    }

    public void contextDestroyed(ServletContextEvent servletContextEvent) {

    }
}
