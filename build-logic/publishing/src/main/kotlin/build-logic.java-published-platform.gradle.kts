plugins {
    id("java-platform")
    id("build-logic.reproducible-builds")
    id("build-logic.publish-to-central")
}

val archivesName = "${isolated.rootProject.name}${path.replace(':', '-')}"
base.archivesName.set(archivesName)

publishing {
    publications {
        create<MavenPublication>("mavenJava") {
            from(components["javaPlatform"])
            artifactId = archivesName
        }
    }
}
