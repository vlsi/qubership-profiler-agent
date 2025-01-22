package org.qubership.profiler.io;

import org.testng.Assert;
import org.testng.annotations.Test;

import java.util.concurrent.TimeUnit;

public class DurationParserTest {
    @Test
    public static void durationMillis() {
        Assert.assertEquals(42, DurationParser.parseDuration("42ms", 0));
        Assert.assertEquals(42, DurationParser.parseDuration("42 ms", 0));
        Assert.assertEquals(42, DurationParser.parseDuration(" 42 MS", 0));
        Assert.assertEquals(42, DurationParser.parseDuration(" 42 ms ", 0));
        Assert.assertEquals(42, DurationParser.parseDuration(" 42 millis ", 0));
    }

    @Test
    public static void durationSeconds() {
        Assert.assertEquals(TimeUnit.SECONDS.toMillis(42), DurationParser.parseDuration("42s", 0));
        Assert.assertEquals(TimeUnit.SECONDS.toMillis(42), DurationParser.parseDuration("42seconds", 0));
    }

    @Test
    public static void durationMinutes() {
        Assert.assertEquals(TimeUnit.MINUTES.toMillis(42), DurationParser.parseDuration("42m", 0));
        Assert.assertEquals(TimeUnit.MINUTES.toMillis(42), DurationParser.parseDuration("42minutes", 0));
        Assert.assertEquals(TimeUnit.MINUTES.toMillis(42), DurationParser.parseDuration("42Min", 0));
    }

    @Test
    public static void durationHours() {
        Assert.assertEquals(TimeUnit.HOURS.toMillis(42), DurationParser.parseDuration("42h", 0));
        Assert.assertEquals(TimeUnit.HOURS.toMillis(42), DurationParser.parseDuration("42hours", 0));
    }

    @Test
    public static void durationDays() {
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42), DurationParser.parseDuration("42", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42), DurationParser.parseDuration("42d", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42), DurationParser.parseDuration("42days", 0));
    }

    @Test
    public static void durationWeek() {
        Assert.assertEquals(TimeUnit.DAYS.toMillis(7), DurationParser.parseDuration("1 week", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(14), DurationParser.parseDuration("2w", 0));
    }

    @Test
    public static void durationMonth() {
        Assert.assertEquals(TimeUnit.DAYS.toMillis(30), DurationParser.parseDuration("1 month", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42 * 30), DurationParser.parseDuration("42mo", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42 * 30), DurationParser.parseDuration("42months", 0));
    }

    @Test
    public static void durationYear() {
        Assert.assertEquals(TimeUnit.DAYS.toMillis(364), DurationParser.parseDuration("1y", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(42 * 364), DurationParser.parseDuration("42years", 0));
    }

    @Test
    public static void durationCombined() {
        Assert.assertEquals(TimeUnit.DAYS.toMillis(2) + TimeUnit.HOURS.toMillis(12) + TimeUnit.MINUTES.toMillis(7), DurationParser.parseDuration("2days 12hours 7m", 0));
        Assert.assertEquals(TimeUnit.DAYS.toMillis(2) - TimeUnit.HOURS.toMillis(12) + TimeUnit.MINUTES.toMillis(7), DurationParser.parseDuration("2days -12hours 7m", 0));
    }

    @Test
    public static void durationUnparsable() {
        Assert.assertEquals(123456, DurationParser.parseDuration("abcde", 123456));
    }
}
