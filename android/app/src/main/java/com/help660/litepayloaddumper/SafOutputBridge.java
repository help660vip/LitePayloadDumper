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
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

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
            bridge.deleteDocuments(new HashSet<>());
            bridge.close();
            throw error;
        }
    }

    String path() {
        return directory.getAbsolutePath();
    }

    void deleteIncomplete(Set<String> completed) {
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
        ParcelFileDescriptor descriptor = resolver.openFileDescriptor(document, "rwt");
        if (descriptor == null) {
            DocumentsContract.deleteDocument(resolver, document);
            throw new IOException("无法打开 " + displayName);
        }
        File link = new File(directory, displayName);
        Os.symlink("/proc/self/fd/" + descriptor.getFd(), link.getAbsolutePath());
        descriptors.add(descriptor);
        documents.put(partition, document);
    }

    private Uri findChild(Uri tree, String displayName) {
        Uri children = DocumentsContract.buildChildDocumentsUriUsingTree(tree, DocumentsContract.getTreeDocumentId(tree));
        String[] projection = {
                DocumentsContract.Document.COLUMN_DOCUMENT_ID,
                DocumentsContract.Document.COLUMN_DISPLAY_NAME
        };
        try (Cursor cursor = resolver.query(children, projection, null, null, null)) {
            if (cursor == null) {
                return null;
            }
            while (cursor.moveToNext()) {
                if (displayName.equals(cursor.getString(1))) {
                    return DocumentsContract.buildDocumentUriUsingTree(tree, cursor.getString(0));
                }
            }
        } catch (Exception ignored) {
            return null;
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
