plugins {
    id("build-logic.java-published-library")
}

dependencies {
    implementation(projects.boot)
    implementation(projects.common)
    implementation(projects.parsers)
    api("jakarta.servlet:jakarta.servlet-api")
    implementation("ch.qos.logback:logback-classic")
    implementation("ch.qos.logback:logback-core")
    implementation("com.fasterxml.jackson.core:jackson-databind")
    implementation("com.google.inject.extensions:guice-servlet")
    implementation("com.google.inject:guice")
    implementation("commons-lang:commons-lang")
    implementation("org.openjdk.jmc:common")
    implementation("org.openjdk.jmc:flightrecorder")
}
