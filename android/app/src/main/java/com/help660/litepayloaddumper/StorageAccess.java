package com.help660.litepayloaddumper;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import mobileapi.Mobileapi;

final class StorageAccess {
    private StorageAccess() {
    }

    static File normalizeWithinRoot(File root, File candidate) throws IOException {
        File normalizedRoot = root.getCanonicalFile();
        File normalizedCandidate = candidate.getCanonicalFile();
        String rootPath = normalizedRoot.getPath();
        String candidatePath = normalizedCandidate.getPath();
        if (!candidatePath.equals(rootPath)
                && !candidatePath.startsWith(rootPath + File.separator)) {
            throw new IOException("保存目录超出共享存储范围");
        }
        return normalizedCandidate;
    }

    static File createChildDirectory(File root, File parent, String name) throws IOException {
        String value = name == null ? "" : name.trim();
        if (value.isEmpty() || value.equals(".") || value.equals("..")
                || value.contains("/") || value.contains("\\")) {
            throw new IOException("文件夹名称无效");
        }
        File normalizedParent = normalizeWithinRoot(root, parent);
        File child = normalizeWithinRoot(root, new File(normalizedParent, value));
        if (child.exists()) {
            if (!child.isDirectory()) {
                throw new IOException("同名文件已经存在");
            }
            return child;
        }
        if (!child.mkdir()) {
            throw new IOException("无法新建文件夹");
        }
        return child;
    }

    static void verifyWritable(File directory) throws Exception {
        if (!directory.exists() && !directory.mkdirs()) {
            throw new IOException("无法创建保存目录");
        }
        if (!directory.isDirectory()) {
            throw new IOException("所选位置不是文件夹");
        }
        String probeName = "LitePayloadDumper-write-test-" + UUID.randomUUID() + ".img";
        File probe = new File(directory, probeName);
        Exception failure = null;
        try {
            Mobileapi.verifyOutputFile(directory.getAbsolutePath(), probeName);
            if (!probe.isFile() || probe.length() != 3) {
                throw new IOException("提取核心没有成功写入测试文件");
            }
        } catch (Exception error) {
            failure = error;
        } finally {
            if (probe.exists() && !probe.delete() && failure == null) {
                failure = new IOException("无法删除目录写入测试文件");
            }
        }
        if (failure != null) {
            throw failure;
        }
    }

    static File createStagingDirectory(File outputDirectory) throws IOException {
        File output = outputDirectory.getCanonicalFile();
        File staging = new File(output, ".LitePayloadDumper-staging-" + UUID.randomUUID()).getCanonicalFile();
        requireOwnedStaging(output, staging);
        if (!staging.mkdir()) {
            throw new IOException("无法创建安全暂存目录");
        }
        return staging;
    }

    static void commitStaging(File outputDirectory, File stagingDirectory, List<String> partitions)
            throws IOException {
        File output = outputDirectory.getCanonicalFile();
        File staging = stagingDirectory.getCanonicalFile();
        requireOwnedStaging(output, staging);
        Map<String, File> backups = new LinkedHashMap<>();
        List<String> committed = new ArrayList<>();
        try {
            for (String partition : partitions) {
                File staged = partitionFile(staging, partition);
                if (!staged.isFile()) {
                    throw new IOException("暂存镜像不存在：" + staged.getName());
                }
            }
            for (String partition : partitions) {
                File destination = partitionFile(output, partition);
                if (destination.exists()) {
                    File backup = new File(staging, partition + ".previous").getCanonicalFile();
                    if (!destination.renameTo(backup)) {
                        throw new IOException("无法准备覆盖 " + destination.getName());
                    }
                    backups.put(partition, backup);
                }
            }
            for (String partition : partitions) {
                File staged = partitionFile(staging, partition);
                File destination = partitionFile(output, partition);
                if (!staged.renameTo(destination)) {
                    throw new IOException("无法保存 " + destination.getName());
                }
                committed.add(partition);
            }
        } catch (IOException error) {
            for (int index = committed.size() - 1; index >= 0; index--) {
                String partition = committed.get(index);
                File destination = partitionFile(output, partition);
                File staged = partitionFile(staging, partition);
                if (!destination.renameTo(staged)) {
                    destination.delete();
                }
            }
            for (Map.Entry<String, File> entry : backups.entrySet()) {
                File destination = partitionFile(output, entry.getKey());
                entry.getValue().renameTo(destination);
            }
            throw error;
        }
        for (File backup : backups.values()) {
            backup.delete();
        }
        if (!staging.delete()) {
            throw new IOException("镜像已保存，但无法删除空的暂存目录");
        }
    }

    static void discardStaging(File outputDirectory, File stagingDirectory) {
        if (stagingDirectory == null) {
            return;
        }
        try {
            File output = outputDirectory.getCanonicalFile();
            File staging = stagingDirectory.getCanonicalFile();
            requireOwnedStaging(output, staging);
            File[] children = staging.listFiles();
            if (children != null) {
                for (File child : children) {
                    if (child.isFile()) {
                        child.delete();
                    }
                }
            }
            staging.delete();
        } catch (IOException ignored) {
        }
    }

    private static File partitionFile(File directory, String partition) throws IOException {
        if (partition == null || partition.isEmpty() || partition.equals(".") || partition.equals("..")
                || partition.contains("/") || partition.contains("\\")) {
            throw new IOException("分区名称无效");
        }
        File file = new File(directory, partition + ".img").getCanonicalFile();
        if (!file.getParentFile().equals(directory.getCanonicalFile())) {
            throw new IOException("分区输出路径无效");
        }
        return file;
    }

    private static void requireOwnedStaging(File output, File staging) throws IOException {
        if (!output.equals(staging.getParentFile())
                || !staging.getName().startsWith(".LitePayloadDumper-staging-")) {
            throw new IOException("暂存目录不安全");
        }
    }
}
