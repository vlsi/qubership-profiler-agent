plugins {
    id("build-logic.java-published-library")
    id("build-logic.test-junit5")
    id("build-logic.test-jmockit")
}

dependencies {
    testImplementation("ch.qos.logback:logback-classic")
}
