# Qubership Profiler Agent

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Netcracker/qubership-profiler-agent/badge)](https://scorecard.dev/viewer/?uri=github.com/Netcracker/qubership-profiler-agent)

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

## Releasing Qubership Profiler

This project defines a [manual release workflow](.github/workflows/release.yaml).

To trigger a release, go to the
👉 [Actions tab → release.yaml](https://github.com/Netcracker/qubership-profiler-agent/actions/workflows/release.yaml) and run it manually.

The release workflow uses [Release Drafter](https://github.com/release-drafter/release-drafter) to prepare
release notes, and it uses labels to group the changes. If you need to adjust the notes, update the labels as needed.

Here's the full step-by-step:
1. Navigate to [Release Workflow](https://github.com/Netcracker/qubership-profiler-agent/actions/workflows/release.yaml)
1. Click on `Run workflow`
1. Select the branch name to be released
1. By default, release workflow would pick the release version from `gradle.properties`, and you can overrided it if needed
1. Click on `Run workflow`

The release workflow would perform the following steps:
1. Check if the release tag `v...` does not exist yet, otherwise it would terminate
1. Bump the version in `gradle.properties` to the release version (e.g., if the manually provided version differs)
1. Build and publish the artifacts to Central Portal
1. Create the release tag and publish GitHub release
1. Bump the version in `gradle.properties` to the next patch version
