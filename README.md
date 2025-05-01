# Qubership Profiler Agent

This repository container the java agent that can attach as a `-javaagent` to the JVM and collect information
as continuous tracing profiler.

* [Qubership Profiler Agent](#qubership-profiler-agent)
  * [Features](#features)
  * [Library and framework instrumentation](#library-and-framework-instrumentation)
  * [How to build](#how-to-build)
    * [Local build](#local-build)

## Features

* Trace for slow requests and errors
* Continuous profiling
* SQL capture (queries and binds)
* Service call capture

## Library and framework instrumentation

Application servers or Portals:

* [Liferay](plugins/liferay)

Build systems:

* ANT
  * [ANT (<=1.10.1)](plugins/ant)
  * [ANT (>=1.10.2)](plugins/ant_1102)

Databases:

* DataStax Cassandra
  * [DataStax Cassandra 3.x](plugins/cassandra)
  * [DataStax cassandra 4.x](plugins/cassandra4)
* [ElasticSearch](plugins/elasticsearch)
* [MySQL JDBC](plugins/mysql)
* [PostgeSQL JDBC](plugins/postgresql)

Distribution tracing:

* [Brave (Zipkin agent)](plugins/brave)
* [Jaeger](plugins/jaeger)
* [Ocelot](plugins/ocelot)
* [Tracing](plugins/tracing)

HTTP clients:

* [HTTP](plugins/http)
* [Java HTTP Client](plugins/java_http_client)
* [Tomcat <= 9.x](plugins/tomcat_http)
* [Tomcat >= 10.x](plugins/tomcat10_http)
* [Undertow < 2.3](plugins/undertow_http)
* [Undertow >= 2.3](plugins/undertow23_http)

Java Frameworks:

* [Apache Felix](plugins/apache_felix)
* [Equinox](plugins/equinox)
* [Spring Framework](plugins/spring)
  * [Spring REST](plugins/springrest)

Loggers:

* [Log4j](plugins/log4j_enhancer)

Other:

* [Jackson](plugins/jackson)
* [Quartz Scheduler](plugins/quartz)
* [Rhino](plugins/rhino)
* [Test](plugins/test)

Queues:

* [ActiveMQ](plugins/activemq)
* [HornetQ](plugins/hornetq)
* [RabbitMQ](plugins/rabbitmq)

## How to build

### Local build

Build requirements:

* Java 17

To build all the artifacts and execute tests, run the following:

```bash
git clone https://github.com/Netcracker/qubership-profiler-agent.git
./gradlew build # builds everything
./gradlew tasks # lists available tasks
```
