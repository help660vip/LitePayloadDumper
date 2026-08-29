package com.help660.litepayloaddumper;

final class Partition {
    final String name;
    final long size;
    final int operations;
    final boolean needsSource;
    final String unsupported;

    Partition(String name, long size, int operations, boolean needsSource, String unsupported) {
        this.name = name;
        this.size = size;
        this.operations = operations;
        this.needsSource = needsSource;
        this.unsupported = unsupported == null ? "" : unsupported;
    }

    boolean extractable() {
        return !needsSource && unsupported.isEmpty();
    }

    String status() {
        if (!unsupported.isEmpty()) {
            return "暂不支持：" + unsupported;
        }
        if (needsSource) {
            return "增量分区（不支持）";
        }
        return operations > 0 ? operations + " 项操作" : "可提取";
    }
}
