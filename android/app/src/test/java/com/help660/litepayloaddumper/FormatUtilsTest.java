package com.help660.litepayloaddumper;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public class FormatUtilsTest {
    @Test public void parsesThreadRange() {
        assertEquals(1, FormatUtils.parseThreads("1"));
        assertEquals(4, FormatUtils.parseThreads(" 4 "));
        assertEquals(64, FormatUtils.parseThreads("64"));
    }

    @Test(expected = IllegalArgumentException.class)
    public void rejectsTooManyThreads() {
        FormatUtils.parseThreads("65");
    }

    @Test public void filtersPartitionNamesWithoutCaseSensitivity() {
        assertTrue(FormatUtils.matches("init_boot", "BOOT"));
        assertTrue(FormatUtils.matches("system", ""));
        assertFalse(FormatUtils.matches("vendor", "boot"));
    }

    @Test public void formatsBytes() {
        assertEquals("512 B", FormatUtils.bytes(512));
        assertEquals("1.0 KB", FormatUtils.bytes(1024));
        assertEquals("1.0 GB", FormatUtils.bytes(1L << 30));
    }
}
