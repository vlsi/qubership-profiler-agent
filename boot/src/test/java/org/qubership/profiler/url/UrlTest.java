package org.qubership.profiler.url;


import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

import java.net.MalformedURLException;
import java.net.URISyntaxException;
import java.net.URL;

public class UrlTest {
    @Test
    public void testJarFile() throws MalformedURLException, URISyntaxException {
        URL u = new URL("jar:file:/Use%20rs/lib/boot.jar!/org/qubership.class");
        String actual = u.toURI().getSchemeSpecificPart();
        assertEquals("file:/Use rs/lib/boot.jar!/org/qubership.class", actual, () -> "URL(" + u + ")");
    }
}
