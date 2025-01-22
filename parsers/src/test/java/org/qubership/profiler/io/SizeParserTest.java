package org.qubership.profiler.io;

import org.testng.Assert;
import org.testng.annotations.Test;

public class SizeParserTest {
    @Test
    public static void sizeBytesTest() {
        Assert.assertEquals(42, SizeParser.parseSize("42b", 0));
        Assert.assertEquals(42, SizeParser.parseSize(" 42 bytes ", 0));
    }

    @Test
    public static void sizeKBytesTest() {
        Assert.assertEquals(42 * 1024, SizeParser.parseSize("42k", 0));
        Assert.assertEquals(42 * 1024, SizeParser.parseSize("42Kb", 0));
    }

    @Test
    public static void sizeMBytesTest() {
        Assert.assertEquals(42 * 1024 * 1024, SizeParser.parseSize("42", 0));
        Assert.assertEquals(42 * 1024 * 1024, SizeParser.parseSize("42 M", 0));
        Assert.assertEquals(42 * 1024 * 1024, SizeParser.parseSize("42 Mo", 0));
    }

    @Test
    public static void sizeGBytesTest() {
        Assert.assertEquals(42L * 1024 * 1024 * 1024, SizeParser.parseSize("42G", 0));
        Assert.assertEquals(42L * 1024 * 1024 * 1024, SizeParser.parseSize("42 gb", 0));
    }

    @Test
    public static void sizeTBytesTest() {
        Assert.assertEquals(42L * 1024 * 1024 * 1024 * 1024, SizeParser.parseSize("42T", 0));
        Assert.assertEquals(42L * 1024 * 1024 * 1024 * 1024, SizeParser.parseSize("42 Tb", 0));
    }

    @Test
    public static void sizeCombined() {
        Assert.assertEquals(42 * 1024 * 1024 + 84 * 1024, SizeParser.parseSize("42M84K", 0));
        Assert.assertEquals(42 * 1024 * 1024 - 84 * 1024, SizeParser.parseSize("42Mb -84Kb", 0));
    }

    @Test
    public static void sizeUnparsable() {
        Assert.assertEquals(12345, SizeParser.parseSize("abcd", 12345));
    }
}
