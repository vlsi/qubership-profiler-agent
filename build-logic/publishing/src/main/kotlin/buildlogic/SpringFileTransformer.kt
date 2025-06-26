package buildlogic

import com.github.jengelman.gradle.plugins.shadow.relocation.RelocateClassContext
import com.github.jengelman.gradle.plugins.shadow.relocation.RelocatePathContext
import com.github.jengelman.gradle.plugins.shadow.transformers.CacheableTransformer
import com.github.jengelman.gradle.plugins.shadow.transformers.Transformer
import com.github.jengelman.gradle.plugins.shadow.transformers.TransformerContext
import org.gradle.api.file.FileTreeElement
import java.io.StringWriter
import java.util.*

@CacheableTransformer
class SpringFileTransformer : Transformer {
    private val transformedProperties = mutableMapOf<String, Properties>()
    private val supportedFiles = setOf(
        "META-INF/spring-autoconfigure-metadata.properties",
        "META-INF/spring.factories",
        "META-INF/spring.handlers",
        "META-INF/spring.schemas",
        "META-INF/spring.tooling",
    )

    override fun getName(): String = "PropertyValuesFileTransformer"

    override fun canTransformResource(element: FileTreeElement): Boolean {
        return element.name in supportedFiles
    }

    override fun transform(context: TransformerContext) {
        // Note: Resources that don't match patternSet should be skipped
        // This is likely already handled by the Shadow plugin based on the canTransformResource method

        // Parse the input file as Properties
        val properties = Properties().apply {
            load(context.`is`)
        }

        // Get or create Properties for this filename
        val path = context.path
        val filename = path
        val props = transformedProperties.getOrPut(filename) { Properties() }

        // Append all values to transformedProperties[filename]
        for (key in properties.stringPropertyNames()) {
            val value = properties.getProperty(key)
            // factories: class=class
            // handlers: url=class
            // schemas: url=path
            // tooling: url=path

            val newKey = when {
                path.endsWith("/spring.factories") -> context.relocateClass(key)
                path.endsWith("/spring-autoconfigure-metadata.properties") -> context.relocateClass(key)
                else -> key
            }
            val newValue = when {
                path.endsWith("/spring.factories") || path.endsWith("/spring.handlers") || path.endsWith("/spring-autoconfigure-metadata.properties") ->
                    value.split(",").joinToString(",") { context.relocateClass(it) }

                path.endsWith("/spring.tooling") && key.endsWith("@icon") -> context.relocatePath(value)
                path.endsWith("/spring.schemas") -> context.relocatePath(value)
                else -> value
            }
            if (!props.containsKey(newKey) || props.getProperty(newKey).isNullOrBlank()) {
                props.setProperty(newKey, newValue)
            } else if (path.endsWith("/spring.factories") || path.endsWith("/spring.handlers") || path.endsWith("/spring-autoconfigure-metadata.properties")) {
                if (newValue.isNotEmpty()) {
                    val prevValue = props.getProperty(newKey)
                    props.setProperty(newKey, "$prevValue,$newValue")
                }
            } else {
                throw IllegalStateException(
                    "Merging values for $path is not supported. Key=$newKey, previousValue=${
                        props.getProperty(newKey)
                    }, newValue=$newValue"
                )
            }
        }
    }

    private fun TransformerContext.relocateClass(className: String): String {
        relocators.forEach { relocator ->
            if (relocator.canRelocateClass(className)) {
                return relocator.relocateClass(RelocateClassContext.builder().className(className).stats(stats).build())
            }
        }
        return className
    }

    private fun TransformerContext.relocatePath(pathName: String): String {
        relocators.forEach { relocator ->
            if (relocator.canRelocatePath(pathName)) {
                return relocator.relocatePath(RelocatePathContext.builder().path(pathName).stats(stats).build())
            }
        }
        return pathName
    }

    override fun hasTransformedResource(): Boolean {
        return transformedProperties.isNotEmpty()
    }

    override fun modifyOutputStream(os: org.apache.tools.zip.ZipOutputStream, preserveFileTimestamps: Boolean) {
        // Iterate over transformedProperties and append the needed zip entries
        for ((path, properties) in transformedProperties) {
            // Create a new zip entry for this path
            val entry = org.apache.tools.zip.ZipEntry(path)
            if (!preserveFileTimestamps) {
                entry.time = 0
            }
            os.putNextEntry(entry)

            // Write the properties to the zip entry
            // Java properties creates a comment with the current date. We remove it to make the file reproducible
            val sw = StringWriter()
            properties.store(sw, null)
            val cleanProperties = sw.toString().lineSequence()
                .filter { !it.startsWith("#") }
                .joinToString(System.lineSeparator())

            os.write(cleanProperties.toByteArray(Charsets.ISO_8859_1))

            // Close the entry
            os.closeEntry()
        }
    }
}
