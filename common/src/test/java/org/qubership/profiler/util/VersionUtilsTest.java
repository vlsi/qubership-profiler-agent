package org.qubership.profiler.util;

import org.testng.Assert;
import org.testng.annotations.Test;

public class VersionUtilsTest {
    @Test
    public void testSimpleVersion() {
        Assert.assertEquals("001.023b002.001", VersionUtils.naturalOrder("1.23b2.1", 3));
    }

    @Test
    public void testNoNumeric() {
        Assert.assertEquals("test", VersionUtils.naturalOrder("test", 5));
    }

    @Test
    public void testEmpty() {
        Assert.assertEquals("", VersionUtils.naturalOrder("", 3));
    }

    @Test
    public void testJustNumber() {
        Assert.assertEquals("123", VersionUtils.naturalOrder("123", 3));
    }

    @Test
    public void testSmallWidth() {
        Assert.assertEquals("1.23b2.1", VersionUtils.naturalOrder("1.23b2.1", 1));
        Assert.assertEquals("01.23b02.01", VersionUtils.naturalOrder("1.23b2.1", 2));
    }

    @Test
    public void testOldFormat() {
        Assert.assertEquals("03.05-00", VersionUtils.naturalOrder("v3.5-0", 2));
    }
}
