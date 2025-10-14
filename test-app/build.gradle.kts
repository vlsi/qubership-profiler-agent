plugins {
    id("build-logic.java-published-library")
}

tasks.jar {
    manifest {
        attributes["Main-Class"] = "com.netcracker.profilerTest.testapp.Main"
    }
}
