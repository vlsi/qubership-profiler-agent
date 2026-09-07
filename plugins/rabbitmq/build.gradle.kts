plugins {
    id("build-logic.profiler-published-plugin")
    id("build-logic.test-junit5")
    id("build-logic.kotlin")
}

dependencies {
    testImplementation(projects.instrumenter)
    testImplementation("org.ow2.asm:asm-tree")
    testImplementation("org.ow2.asm:asm-util")
}

// ChannelNInstrumentationTest transforms the real ChannelN of each version listed here. amqp-client
// 5.31.0 added basicPublish(String, String, boolean, boolean, BasicProperties, ByteBuffer,
// WriteListener), so 5.30.0 and 5.31.0 are the two sides of that boundary and the injector has to
// cover both. Renovate ignores plugins/*/build.gradle.kts (see ignorePaths in renovate.json), so
// these versions stay where they are put, and the version that moves comes from bom-testing below.
val pinnedAmqpClientVersions = listOf("5.30.0", "5.31.0")

val amqpClientClasspaths = buildList {
    for (version in pinnedAmqpClientVersions) {
        val suffix = version.replace('.', '_')
        val declared = configurations.dependencyScope("amqpClient$suffix") {
            description = "Declares amqp-client $version, instrumented by ChannelNInstrumentationTest"
        }
        add(
            configurations.resolvable("amqpClient${suffix}Classpath") {
                extendsFrom(declared.get())
            }
        )
        dependencies.add(declared.name, "com.rabbitmq:amqp-client:$version")
    }
    val declaredCurrent = configurations.dependencyScope("amqpClientCurrent") {
        description = "Declares the amqp-client version bom-testing pins, so Renovate keeps it moving"
    }
    add(
        configurations.resolvable("amqpClientCurrentClasspath") {
            extendsFrom(declaredCurrent.get())
        }
    )
    dependencies.add(declaredCurrent.name, dependencies.platform(projects.bomTesting))
    dependencies.add(declaredCurrent.name, "com.rabbitmq:amqp-client")
}

tasks.test {
    systemProperty("amqp.client.classpath.count", amqpClientClasspaths.size)
    // The jars are handed to the test as paths rather than added to its own classpath: the test
    // reads bytecode from several amqp-client versions at once, which one classpath cannot hold.
    // @Classpath on the provider declares them as an input of this task, so no inputs.files is
    // needed, and the provider holds a file collection rather than a script reference, which is what
    // the configuration cache can serialize.
    amqpClientClasspaths.forEachIndexed { index, classpath ->
        jvmArgumentProviders.add(
            objects.newInstance<AmqpClientClasspath>().apply {
                this.index.set(index)
                jars.from(classpath)
            }
        )
    }
}

/** Passes one amqp-client version's jars to the test as `-Damqp.client.classpath.<index>`. */
abstract class AmqpClientClasspath : CommandLineArgumentProvider {
    @get:Input
    abstract val index: Property<Int>

    @get:Classpath
    abstract val jars: ConfigurableFileCollection

    override fun asArguments(): Iterable<String> =
        listOf("-Damqp.client.classpath.${index.get()}=${jars.asPath}")
}
