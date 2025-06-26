import buildlogic.SpringFileTransformer
import com.github.jengelman.gradle.plugins.shadow.transformers.ServiceFileTransformer

plugins {
    id("build-logic.java-published-library")
    id("com.gradleup.shadow")
}

dependencies {
    implementation(projects.dumper)
    implementation(projects.common)
    implementation(projects.web)
    implementation(projects.cli)
    implementation("org.slf4j:slf4j-api")
    implementation("com.github.ziplet:ziplet")
    implementation("stax:stax-api")
}

tasks.shadowJar {
    mergeServiceFiles()
    // spring-configuration-metadata.json is meant for IDEs
    exclude("META-INF/spring-configuration-metadata.json")
    transform(SpringFileTransformer::class.java)
    transform(ServiceFileTransformer::class.java) {
        include("META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports")
    }
    relocate("ch.qos.logback", "org.qubership.profiler.shaded.ch.qos.logback")
    relocate("com.fasterxml", "org.qubership.profiler.shaded.com.fasterxml")
    relocate("com.github.ziplet", "org.qubership.profiler.shaded.com.github.ziplet")
    relocate("com.google", "org.qubership.profiler.shaded.com.google")
    relocate("com.graphbuilder", "org.qubership.profiler.shaded.com.graphbuilder")
    relocate("com.microsoft.schemas", "org.qubership.profiler.shaded.com.microsoft.schemas")
    relocate("gnu.trove", "org.qubership.profiler.shaded.gnu.trove")
    relocate("net.sourceforge.argparse4j", "org.qubership.profiler.shaded.net.sourceforge.argparse4j")
    relocate("org.HdrHistogram", "org.qubership.profiler.shaded.org.HdrHistogram")
    relocate("org.aopalliance", "org.qubership.profiler.shaded.org.aopalliance")
    relocate("org.apache", "org.qubership.profiler.shaded.org.apache")
    relocate("org.etsi.uri", "org.qubership.profiler.shaded.org.etsi.uri")
    relocate("org.objectweb", "org.qubership.profiler.shaded.org.objectweb")
    relocate("org.openjdk.jmc", "org.qubership.profiler.shaded.org.openjdk.jmc")
    relocate("org.openxmlformats", "org.qubership.profiler.shaded.org.openxmlformats")
    relocate("org.slf4j", "org.qubership.profiler.shaded.org.slf4j")
    relocate("org.springframework", "org.qubership.profiler.shaded.org.springframework")
    relocate("org.w3.x2000", "org.qubership.profiler.shaded.org.w3.x2000")
    relocate("schemaorg_apache_xmlbeans", "org.qubership.profiler.shaded.schemaorg_apache_xmlbeans")
}
