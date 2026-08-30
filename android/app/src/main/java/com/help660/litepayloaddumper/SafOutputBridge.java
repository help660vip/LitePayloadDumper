package com.help660.litepayloaddumper;

import android.content.ContentResolver;
import android.content.Context;
import android.database.Cursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.DocumentsContract;
import android.system.Os;

import java.io.Closeable;
import java.io.File;
import java.io.IOException;
import java.io.RandomAccessFile;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

import mobileapi.Mobileapi;

final class SafOutputBridge implements Closeable {
    private final ContentResolver resolver;
    private final File directory;
    private final List<ParcelFileDescriptor> descriptors = new ArrayList<>();
    private final Map<String, Uri> documents = new HashMap<>();

    private SafOutputBridge(Context context) throws IOException {
        resolver = context.getContentResolver();
        directory = new File(context.getCacheDir(), "output-bridge-" + UUID.randomUUID());
        if (!directory.mkdirs()) {
            throw new IOException("无法创建输出桥接目录") ;
        }
    }

    static SafOutputBridge create(Context context, Uri tree, List<String> partitions) throws Exception {
        SafOutputBridge bridge = new SafOutputBridge(context);
        try {
            for (String partition : partitions) {
                bridge.add(tree, partition);
            }
            return bridge;
        } catch (Exception error) {
            bridge.close();
            bridge.deleteDocuments(new HashSet<>());
            throw error;
        }
    }

    static void verifyWritable(Context context, Uri tree) throws Exception {
        if (tree == null || !"content".equals(tree.getScheme())) {
            throw new IOException("所选位置不是可写目录");
        }
        String probeName = "LitePayloadDumper-write-test-" + UUID.randomUUID();
        List<String> probes = new ArrayList<>();
        probes.add(probeName);
        SafOutputBridge bridge = null;
        Exception failure = null;
        try {
            bridge = create(context, tree, probes);
            Mobileapi.verifyOutputFile(bridge.path(), probeName + ".img");
        } catch (SecurityException error) {
            failure = new IOException("目录写入授权无效", error);
        } catch (Exception error) {
            failure = error;
        } finally {
            if (bridge != null) {
                bridge.close();
                Uri document = bridge.documents.get(probeName);
                if (document != null) {
                    try {
                        if (DocumentsContract.deleteDocument(bridge.resolver, document)) {
                            bridge.documents.remove(probeName);
                        } else if (failure == null) {
                            failure = new IOException("目录提供方不允许删除测试文件");
                        }
                    } catch (Exception error) {
                        if (failure == null) {
                            failure = new IOException("目录删除授权无效", error);
                        }
                    }
                }
            }
        }
        if (failure != null) {
            throw failure;
        }
    }

    String path() {
        return directory.getAbsolutePath();
    }

    void deleteIncomplete(Set<String> completed) {
        close();
        deleteDocuments(completed);
    }

    private void add(Uri tree, String partition) throws Exception {
        String displayName = partition + ".img";
        Uri existing = findChild(tree, displayName);
        if (existing != null && !DocumentsContract.deleteDocument(resolver, existing)) {
            throw new IOException("无法覆盖 " + displayName);
        }
        Uri parent = DocumentsContract.buildDocumentUriUsingTree(tree, DocumentsContract.getTreeDocumentId(tree));
        Uri document = DocumentsContract.createDocument(resolver, parent, "application/octet-stream", displayName);
        if (document == null) {
            throw new IOException("无法创建 " + displayName);
        }
        ParcelFileDescriptor descriptor = resolver.openFileDescriptor(document, "rw");
        if (descriptor == null) {
            DocumentsContract.deleteDocument(resolver, document);
            throw new IOException("无法打开 " + displayName);
        }
        File link = new File(directory, displayName);
        try {
            Os.symlink("/proc/self/fd/" + descriptor.getFd(), link.getAbsolutePath());
            try (RandomAccessFile output = new RandomAccessFile(link, "rw")) {
                output.setLength(0);
            }
        } catch (Exception error) {
            link.delete();
            descriptor.close();
            DocumentsContract.deleteDocument(resolver, document);
            throw new IOException("所选目录不支持写入 " + displayName, error);
        }
        descriptors.add(descriptor);
        documents.put(partition, document);
    }

    private Uri findChild(Uri tree, String displayName) throws Exception {
        Uri children = DocumentsContract.buildChildDocumentsUriUsingTree(tree, DocumentsContract.getTreeDocumentId(tree));
        String[] projection = {
                DocumentsContract.Document.COLUMN_DOCUMENT_ID,
                DocumentsContract.Document.COLUMN_DISPLAY_NAME
        };
        try (Cursor cursor = resolver.query(children, projection, null, null, null)) {
            if (cursor == null) {
                throw new IOException("无法读取保存目录");
            }
            while (cursor.moveToNext()) {
                if (displayName.equals(cursor.getString(1))) {
                    return DocumentsContract.buildDocumentUriUsingTree(tree, cursor.getString(0));
                }
            }
        }
        return null;
    }

    private void deleteDocuments(Set<String> keep) {
        for (Map.Entry<String, Uri> entry : documents.entrySet()) {
            if (!keep.contains(entry.getKey())) {
                try {
                    DocumentsContract.deleteDocument(resolver, entry.getValue());
                } catch (Exception ignored) {
                }
            }
        }
    }

    @Override
    public void close() {
        for (ParcelFileDescriptor descriptor : descriptors) {
            try {
                descriptor.close();
            } catch (IOException ignored) {
            }
        }
        descriptors.clear();
        File[] children = directory.listFiles();
        if (children != null) {
            for (File child : children) {
                child.delete();
            }
        }
        directory.delete();
    }
}
