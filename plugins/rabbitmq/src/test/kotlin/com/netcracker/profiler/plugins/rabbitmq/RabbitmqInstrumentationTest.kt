package com.netcracker.profiler.plugins.rabbitmq

import com.netcracker.profiler.testkit.instrumentation.LibraryUnderTest
import com.netcracker.profiler.testkit.instrumentation.PluginInstrumentation

import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource

/**
 * Checks this plugin's injectors against the real amqp-client and spring-rabbit, at the versions
 * `build.gradle.kts` lists.
 *
 * `ChannelN.basicPublish` gained a `ByteBuffer` overload in amqp-client 5.31.0, which the rule in
 * `rabbitmq.xml` matches like the `byte[]` one. The profiler builds the descriptor of an
 * `execute-after` call from the parameter types of the method it matched, so the two overloads call
 * two different `basicPublish$profiler` descriptors, and an injector that declares only one of them
 * throws `NoSuchMethodError` when the instrumented method first runs.
 *
 * The cases name no class of their own: [PluginInstrumentation] reads `rabbitmq.xml` and checks every
 * class it selects, so a rule added to that file is covered without an edit here.
 */
class RabbitmqInstrumentationTest {
    @ParameterizedTest
    @MethodSource("libraries")
    fun `every reference the instrumentation adds is declared`(library: LibraryUnderTest) =
        PluginInstrumentation.assertAddedReferencesResolve(library)

    @ParameterizedTest
    @MethodSource("libraries")
    fun `every instrumented class passes bytecode verification`(library: LibraryUnderTest) =
        PluginInstrumentation.assertInstrumentedClassesVerify(library)

    @ParameterizedTest
    @MethodSource("libraries")
    fun `every method the configuration selects is instrumented`(library: LibraryUnderTest) =
        PluginInstrumentation.assertSelectedMethodsAreInstrumented(library)

    companion object {
        @JvmStatic
        fun libraries() = PluginInstrumentation.librariesUnderTest()
    }
}
