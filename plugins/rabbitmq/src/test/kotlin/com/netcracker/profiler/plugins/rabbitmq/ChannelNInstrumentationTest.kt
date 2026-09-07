package com.netcracker.profiler.plugins.rabbitmq

import com.netcracker.profiler.agent.ProfilingTransformer
import com.netcracker.profiler.agent.plugins.EnhancerRegistryPluginImpl
import com.netcracker.profiler.configuration.ConfigurationImpl
import com.netcracker.profiler.instrument.enhancement.EnhancerPlugin_rabbitmq

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.objectweb.asm.ClassReader
import org.objectweb.asm.Handle
import org.objectweb.asm.Opcodes
import org.objectweb.asm.Type
import org.objectweb.asm.tree.ClassNode
import org.objectweb.asm.tree.FieldInsnNode
import org.objectweb.asm.tree.InvokeDynamicInsnNode
import org.objectweb.asm.tree.MethodInsnNode
import org.objectweb.asm.tree.TypeInsnNode
import org.objectweb.asm.util.CheckClassAdapter

import java.io.ByteArrayOutputStream
import java.io.File
import java.io.PrintWriter
import java.lang.reflect.Modifier
import java.net.URLClassLoader
import java.nio.file.Path
import java.nio.file.Paths

/**
 * Fails when the injector declares no `basicPublish$profiler` for a `basicPublish` overload that a
 * supported amqp-client version declares.
 *
 * `ExecuteMethod` builds the descriptor of an `execute-after` call from the parameter types of the
 * instrumented method, so the `byte[]` and the `ByteBuffer` form of `ChannelN.basicPublish` call two
 * different `basicPublish$profiler` descriptors. A descriptor no method matches is resolved when the
 * instrumented method first runs, and throws `NoSuchMethodError` there. Bytecode verification does
 * not resolve members, so nothing between the transformation and that first call reports it, which
 * is why `the instrumented ChannelN passes bytecode verification` stays green on a broken injector.
 *
 * amqp-client 5.31.0 added `basicPublish(String, String, boolean, boolean, BasicProperties,
 * ByteBuffer, WriteListener)`; earlier versions publish a `byte[]` body only. The versions under
 * test are declared in `plugins/rabbitmq/build.gradle.kts`, and Gradle passes their jars in the
 * `amqp.client.classpath.*` system properties. When a new overload fails this test, add the method
 * it calls to `src/injector/java/com/rabbitmq/client/impl/ChannelN.java`.
 *
 * [expectedProfilerCalls] restates the rule in `config/minimal/50main/rabbitmq.xml`, so a change to
 * that rule fails this test until the expectation follows it.
 */
class ChannelNInstrumentationTest {
    @ParameterizedTest
    @MethodSource("amqpClientDistributions")
    fun `every basicPublish overload the rule matches gets an execute-after call`(classpath: List<Path>) {
        withClassLoader(classpath) { loader ->
            val original = classNode(readClassFile(loader, CHANNEL_N))
            val instrumented = instrument(loader)
            val expected = expectedProfilerCalls(original).sorted()

            // Both sides are empty when the rule reaches nothing, and the plugin then profiles nothing.
            assertNotEquals(
                emptyList<String>(),
                expected,
                "basicPublish overloads in $CHANNEL_N that the rule in $RABBITMQ_CONFIG matches"
            )
            assertEquals(
                expected,
                references(instrumented)
                    .filter { it.owner == CHANNEL_N && it.name == "basicPublish\$profiler" }
                    .map { it.descriptor }
                    .sorted(),
                "basicPublish\$profiler descriptors called by the instrumented $CHANNEL_N"
            )
        }
    }

    @ParameterizedTest
    @MethodSource("amqpClientDistributions")
    fun `every reference the instrumentation adds to ChannelN resolves`(classpath: List<Path>) {
        withClassLoader(classpath) { loader ->
            val original = readClassFile(loader, CHANNEL_N)
            val instrumented = instrument(loader)
            val added = references(instrumented) - references(original)
            val instrumentedNode = classNode(instrumented)

            assertEquals(
                emptyList<String>(),
                added.mapNotNull { unresolved(it, instrumentedNode, loader) },
                "references the profiler added to $CHANNEL_N"
            )
        }
    }

    @ParameterizedTest
    @MethodSource("amqpClientDistributions")
    fun `the instrumented ChannelN passes bytecode verification`(classpath: List<Path>) {
        withClassLoader(classpath) { loader ->
            val instrumented = instrument(loader)
            val report = ByteArrayOutputStream()
            PrintWriter(report, true).use {
                CheckClassAdapter.verify(ClassReader(instrumented), loader, false, it)
            }
            assertEquals("", report.toString("UTF-8"), "CheckClassAdapter.verify($CHANNEL_N)")
        }
    }

    /**
     * Negative control for [unresolved]: a resolver that reported every input as resolved, or that
     * gave every failure the same reason, would pass every other test in this class.
     */
    @ParameterizedTest
    @MethodSource("fabricatedReferences")
    fun `a reference is reported with the reason it cannot be resolved`(reference: Reference, reason: String?) {
        assertEquals(reason, unresolved(reference, ClassNode(), javaClass.classLoader), "unresolved($reference)")
    }

    /** Transforms the real `ChannelN` on [loader], failing the test when no rule selects it. */
    private fun instrument(loader: ClassLoader): ByteArray {
        val transformed = transformer.transform(loader, CHANNEL_N, null, null, readClassFile(loader, CHANNEL_N))
        assertNotNull(transformed, "ProfilingTransformer.transform(\"$CHANNEL_N\")")
        return transformed!!
    }

    companion object {
        private const val CHANNEL_N = "com/rabbitmq/client/impl/ChannelN"
        private const val RABBITMQ_CONFIG = "config/minimal/50main/rabbitmq.xml"
        private val THROWABLE = Type.getObjectType("java/lang/Throwable")
        private val STRING = Type.getObjectType("java/lang/String")

        /**
         * The transformer the agent builds from this plugin's own configuration and enhancers.
         *
         * Both are registered in a process-wide registry [EnhancerRegistryPluginImpl] holds, so this
         * runs once however many tests and parameters ask for it.
         */
        private val transformer: ProfilingTransformer by lazy {
            EnhancerRegistryPluginImpl()
            EnhancerPlugin_rabbitmq()
            ProfilingTransformer(ConfigurationImpl(resourcePath(RABBITMQ_CONFIG).toString()))
        }

        /**
         * References [unresolved] has to tell apart, with the line each one owes the failure report.
         *
         * `Profiler.event(Object, String)` is the static method every injected body calls; the
         * profiler's own runtime is on the test classpath, so it stands for a class that loads.
         */
        @JvmStatic
        fun fabricatedReferences(): List<Arguments> {
            val profiler = "com/netcracker/profiler/agent/Profiler"
            val event = "(Ljava/lang/Object;Ljava/lang/String;)V"
            return listOf(
                Arguments.argumentSet(
                    "class not found",
                    Reference(ReferenceKind.METHOD, "com/does/not/Exist", "publish", "()V", static = false),
                    "com/does/not/Exist.publish()V (class not found)"
                ),
                Arguments.argumentSet(
                    "no such member",
                    Reference(ReferenceKind.METHOD, profiler, "publish", "()V", static = true),
                    "$profiler.publish()V (no such member)"
                ),
                Arguments.argumentSet(
                    "static mismatch",
                    Reference(ReferenceKind.METHOD, profiler, "event", event, static = false),
                    "$profiler.event$event (static mismatch)"
                ),
                Arguments.argumentSet(
                    "resolved",
                    Reference(ReferenceKind.METHOD, profiler, "event", event, static = true),
                    null
                )
            )
        }

        @JvmStatic
        fun amqpClientDistributions(): List<Arguments> {
            val count = System.getProperty("amqp.client.classpath.count")
                ?: error("amqp.client.classpath.count is not set; run this test through Gradle")
            return (0 until count.toInt()).map { index ->
                val classpath = System.getProperty("amqp.client.classpath.$index")
                    .split(File.pathSeparator)
                    .map { Paths.get(it) }
                Arguments.argumentSet(amqpClientVersion(classpath), classpath)
            }
        }

        /**
         * The `basicPublish$profiler` descriptors the rule in `rabbitmq.xml` calls for, read from
         * [original] rather than from the injector.
         *
         * The rule matches `basicPublish(java.lang.String, java.lang.String, boolean, boolean, ...)`
         * and passes `(p1, p2, p6, throwable)`, and `ExecuteMethod` types each argument as the
         * matched method types it, so an overload's sixth parameter decides its descriptor.
         */
        private fun expectedProfilerCalls(original: ClassNode): List<String> =
            original.methods
                .filter { it.name == "basicPublish" }
                .map { Type.getArgumentTypes(it.desc) }
                .filter {
                    it.size >= 6 &&
                        it[0] == STRING && it[1] == STRING &&
                        it[2] == Type.BOOLEAN_TYPE && it[3] == Type.BOOLEAN_TYPE
                }
                .map { Type.getMethodDescriptor(Type.VOID_TYPE, STRING, STRING, it[5], THROWABLE) }
                .distinct()

        /** Returns the version of the `amqp-client` jar on [classpath], for the parameterized case name. */
        private fun amqpClientVersion(classpath: List<Path>): String =
            classpath.asSequence()
                .map { it.fileName.toString() }
                .mapNotNull { Regex("amqp-client-(.+)\\.jar").matchEntire(it) }
                .map { it.groupValues[1] }
                .firstOrNull()
                ?: error("no amqp-client jar on $classpath")

        private fun resourcePath(name: String): Path =
            Paths.get(
                (ChannelNInstrumentationTest::class.java.classLoader.getResource(name)
                    ?: error("$name is not on the test classpath")).toURI()
            )

        private fun <T> withClassLoader(classpath: List<Path>, body: (ClassLoader) -> T): T =
            URLClassLoader(
                classpath.map { it.toUri().toURL() }.toTypedArray(),
                ChannelNInstrumentationTest::class.java.classLoader
            ).use(body)

        private fun readClassFile(loader: ClassLoader, internalName: String): ByteArray =
            (loader.getResourceAsStream("$internalName.class")
                ?: error("$internalName.class is not on $loader")).use { it.readBytes() }

        private fun classNode(classFile: ByteArray): ClassNode =
            ClassNode().also { ClassReader(classFile).accept(it, ClassReader.SKIP_FRAMES) }

        /**
         * Collects the method, field and type instructions of every body in [classFile], and the
         * handles an `invokedynamic` names: with `ADD_INDY_TRY_CATCH_BLOCKS` on, which is the
         * default, an `execute-after` call reaches its target through `invokedynamic` rather than
         * through `invokevirtual`, and the target then appears only among the bootstrap arguments.
         *
         * The handler type of a `try`/`catch` is collected too, since `ProfileMethodAdapter` adds a
         * `java/lang/Throwable` handler to every instrumented method other than a constructor.
         *
         * Left out: the types an injected member's own descriptor and signature name, such as the
         * `java/nio/ByteBuffer` parameter of `basicPublish$profiler`, which a call site names again
         * where it matters; and, because the transformation adds none of them today, a class or
         * method-type constant loaded with `LDC`, the element type of a `MULTIANEWARRAY`, and the
         * bootstrap arguments of a `ConstantDynamic`.
         */
        private fun references(classFile: ByteArray): Set<Reference> {
            val references = LinkedHashSet<Reference>()
            for (method in classNode(classFile).methods) {
                for (instruction in method.instructions) {
                    when (instruction) {
                        is MethodInsnNode -> references += Reference(
                            ReferenceKind.METHOD, instruction.owner, instruction.name, instruction.desc,
                            static = instruction.opcode == Opcodes.INVOKESTATIC
                        )
                        is FieldInsnNode -> references += Reference(
                            ReferenceKind.FIELD, instruction.owner, instruction.name, instruction.desc,
                            static = instruction.opcode == Opcodes.GETSTATIC ||
                                instruction.opcode == Opcodes.PUTSTATIC
                        )
                        is TypeInsnNode -> references += Reference(ReferenceKind.TYPE, instruction.desc)
                        is InvokeDynamicInsnNode -> {
                            references += Reference(instruction.bsm)
                            instruction.bsmArgs.filterIsInstance<Handle>()
                                .forEach { references += Reference(it) }
                        }
                    }
                }
                for (handler in method.tryCatchBlocks) {
                    handler.type?.let { references += Reference(ReferenceKind.TYPE, it) }
                }
            }
            return references
        }

        /**
         * Returns why [reference] cannot be resolved against [loader], or null when it resolves.
         *
         * A member the enhancer injected is declared by [instrumented] and by nothing on [loader],
         * so the transformed class is searched before the loaded one. Declaration and staticness are
         * modelled; access control is not. An injected body does name members outside the enhanced
         * class, `ByteBuffer.duplicate` and `Math.min` among them, but those are public, and a member
         * the injector declares is reached by the search of [instrumented] whatever its access, so a
         * package-private or protected target does not arise here.
         */
        private fun unresolved(reference: Reference, instrumented: ClassNode, loader: ClassLoader): String? {
            if (reference.kind == ReferenceKind.TYPE) {
                val element = when {
                    reference.owner.startsWith("[") -> Type.getType(reference.owner).elementType
                    else -> Type.getObjectType(reference.owner)
                }
                if (element.sort != Type.OBJECT || loads(element.internalName, loader) != null) {
                    return null
                }
                return "$reference (class not found)"
            }
            if (reference.owner.startsWith("[")) {
                // An array declares no member of its own; the reference is to one java.lang.Object declares.
                return null
            }
            if (reference.owner == instrumented.name) {
                when (declaredBy(instrumented, reference)) {
                    Match.FOUND -> return null
                    Match.STATIC_MISMATCH -> return "$reference (static mismatch)"
                    Match.ABSENT -> Unit
                }
            }
            val owner = loads(reference.owner, loader) ?: return "$reference (class not found)"
            return when (search(owner, reference)) {
                Match.FOUND -> null
                Match.STATIC_MISMATCH -> "$reference (static mismatch)"
                Match.ABSENT -> "$reference (no such member)"
            }
        }

        private fun loads(internalName: String, loader: ClassLoader): Class<*>? =
            try {
                Class.forName(internalName.replace('/', '.'), false, loader)
            } catch (notFound: ClassNotFoundException) {
                null
            } catch (broken: LinkageError) {
                null
            }

        private fun declaredBy(node: ClassNode, reference: Reference): Match {
            val access: List<Int> = when (reference.kind) {
                ReferenceKind.METHOD -> node.methods
                    .filter { it.name == reference.name && it.desc == reference.descriptor }
                    .map { it.access }
                ReferenceKind.FIELD -> node.fields
                    .filter { it.name == reference.name && it.desc == reference.descriptor }
                    .map { it.access }
                ReferenceKind.TYPE -> emptyList()
            }
            if (access.isEmpty()) {
                return Match.ABSENT
            }
            return match(access.map { (it and Opcodes.ACC_STATIC) != 0 }, reference)
        }

        private fun search(owner: Class<*>, reference: Reference): Match {
            if (reference.name == "<init>") {
                val found = owner.declaredConstructors
                    .filter { Type.getConstructorDescriptor(it) == reference.descriptor }
                return if (found.isEmpty()) Match.ABSENT else Match.FOUND
            }
            val pending = ArrayDeque<Class<*>>()
            pending.add(owner)
            val seen = HashSet<Class<*>>()
            var result = Match.ABSENT
            while (pending.isNotEmpty()) {
                val type = pending.removeFirst()
                if (!seen.add(type)) {
                    continue
                }
                val declared = when (reference.kind) {
                    ReferenceKind.METHOD -> type.declaredMethods
                        .filter { it.name == reference.name && Type.getMethodDescriptor(it) == reference.descriptor }
                        .map { it.modifiers }
                    ReferenceKind.FIELD -> type.declaredFields
                        .filter { it.name == reference.name && Type.getDescriptor(it.type) == reference.descriptor }
                        .map { it.modifiers }
                    ReferenceKind.TYPE -> emptyList()
                }
                // A private member is not inherited, so one declared above the owner is not this reference's.
                val modifiers = declared.filter { type == owner || !Modifier.isPrivate(it) }
                if (modifiers.isNotEmpty()) {
                    val verdict = match(modifiers.map { Modifier.isStatic(it) }, reference)
                    if (verdict == Match.FOUND) {
                        return Match.FOUND
                    }
                    result = verdict
                }
                type.superclass?.let { pending.add(it) }
                pending.addAll(type.interfaces)
            }
            return result
        }

        private fun match(staticness: List<Boolean>, reference: Reference): Match =
            if (staticness.any { it == reference.static }) Match.FOUND else Match.STATIC_MISMATCH

        private enum class Match { FOUND, STATIC_MISMATCH, ABSENT }
    }

    enum class ReferenceKind { METHOD, FIELD, TYPE }

    data class Reference(
        val kind: ReferenceKind,
        val owner: String,
        val name: String = "",
        val descriptor: String = "",
        val static: Boolean = false,
    ) {
        constructor(handle: Handle) : this(
            if (handle.tag >= Opcodes.H_INVOKEVIRTUAL) ReferenceKind.METHOD else ReferenceKind.FIELD,
            handle.owner,
            handle.name,
            handle.desc,
            static = handle.tag == Opcodes.H_INVOKESTATIC || handle.tag == Opcodes.H_GETSTATIC ||
                handle.tag == Opcodes.H_PUTSTATIC,
        )

        override fun toString(): String = when (kind) {
            ReferenceKind.METHOD -> "$owner.$name$descriptor"
            ReferenceKind.FIELD -> "$owner.$name:$descriptor"
            ReferenceKind.TYPE -> owner
        }
    }
}
