package com.help660.litepayloaddumper;

import java.util.Locale;

final class FormatUtils {
    private FormatUtils() {}

    static int parseThreads(String text) {
        final int value;
        try {
            value = Integer.parseInt(text == null ? "" : text.trim());
        } catch (NumberFormatException error) {
            throw new IllegalArgumentException("线程数必须是 1～64 的整数");
        }
        if (value < 1 || value > 64) {
            throw new IllegalArgumentException("线程数必须是 1～64 的整数");
        }
        return value;
    }

    static String bytes(long value) {
        if (value < 1024) {
            return value + " B";
        }
        String[] units = {"KB", "MB", "GB", "TB"};
        double size = value;
        int unit = -1;
        while (size >= 1024 && unit < units.length - 1) {
            size /= 1024;
            unit++;
        }
        if (size >= 100) {
            return String.format(Locale.US, "%.0f %s", size, units[unit]);
        }
        return String.format(Locale.US, "%.1f %s", size, units[unit]);
    }

    static boolean matches(String name, String query) {
        String needle = query == null ? "" : query.trim().toLowerCase(Locale.ROOT);
        return needle.isEmpty() || name.toLowerCase(Locale.ROOT).contains(needle);
    }
}
