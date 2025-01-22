package org.qubership.profiler.url;

import org.testng.Assert;
import org.testng.annotations.Test;

import java.net.MalformedURLException;
import java.net.URISyntaxException;
import java.net.URL;

public class UrlTest {
    @Test
    public void testJarFile() throws MalformedURLException, URISyntaxException {
        URL u = new URL("jar:file:/Use%20rs/lib/boot.jar!/org/qubership.class");
        String actual = u.toURI().getSchemeSpecificPart();
        Assert.assertEquals(actual, "file:/Use rs/lib/boot.jar!/org/qubership.class", "URL(" + u.toString() + ")");
    }
}
