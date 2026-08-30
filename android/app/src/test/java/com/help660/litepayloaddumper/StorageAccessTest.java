package com.help660.litepayloaddumper;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.Arrays;

public class StorageAccessTest {
    @Test
    public void normalizesDirectoriesInsideSharedStorage() throws Exception {
        File root = Files.createTempDirectory("lpd-storage-root").toFile();
        File nested = new File(root, "Download/Images");
        assertTrue(nested.mkdirs());

        assertEquals(nested.getCanonicalFile(), StorageAccess.normalizeWithinRoot(root, nested));
    }

    @Test
    public void rejectsDirectoriesOutsideSharedStorage() throws Exception {
        File root = Files.createTempDirectory("lpd-storage-root").toFile();
        File outside = Files.createTempDirectory("lpd-storage-outside").toFile();

        assertThrows(IOException.class, () -> StorageAccess.normalizeWithinRoot(root, outside));
    }

    @Test
    public void rejectsUnsafeChildNames() throws Exception {
        File root = Files.createTempDirectory("lpd-storage-root").toFile();

        assertThrows(IOException.class, () -> StorageAccess.createChildDirectory(root, root, "../escape"));
        assertThrows(IOException.class, () -> StorageAccess.createChildDirectory(root, root, "a/b"));
    }

    @Test
    public void discardOnlyRemovesOwnedStagingFiles() throws Exception {
        File output = Files.createTempDirectory("lpd-output").toFile();
        File existing = new File(output, "boot.img");
        Files.write(existing.toPath(), "old".getBytes(StandardCharsets.UTF_8));
        File staging = StorageAccess.createStagingDirectory(output);
        Files.write(new File(staging, "boot.img").toPath(), "partial".getBytes(StandardCharsets.UTF_8));

        StorageAccess.discardStaging(output, staging);

        assertEquals("old", new String(Files.readAllBytes(existing.toPath()), StandardCharsets.UTF_8));
        assertFalse(staging.exists());
    }

    @Test
    public void commitReplacesFilesOnlyAfterEveryStagedImageExists() throws Exception {
        File output = Files.createTempDirectory("lpd-output").toFile();
        File existing = new File(output, "boot.img");
        Files.write(existing.toPath(), "old".getBytes(StandardCharsets.UTF_8));
        File staging = StorageAccess.createStagingDirectory(output);
        Files.write(new File(staging, "boot.img").toPath(), "new".getBytes(StandardCharsets.UTF_8));

        assertThrows(IOException.class, () -> StorageAccess.commitStaging(
                output, staging, Arrays.asList("boot", "init_boot")));
        assertEquals("old", new String(Files.readAllBytes(existing.toPath()), StandardCharsets.UTF_8));

        Files.write(new File(staging, "init_boot.img").toPath(), "init".getBytes(StandardCharsets.UTF_8));
        StorageAccess.commitStaging(output, staging, Arrays.asList("boot", "init_boot"));
        assertEquals("new", new String(Files.readAllBytes(existing.toPath()), StandardCharsets.UTF_8));
        assertFalse(staging.exists());
    }
}
