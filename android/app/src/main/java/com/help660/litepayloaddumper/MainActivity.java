package com.help660.litepayloaddumper;

import android.Manifest;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.graphics.Color;
import android.graphics.Typeface;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import android.provider.Settings;
import android.system.Os;
import android.system.OsConstants;
import android.text.Editable;
import android.text.InputType;
import android.text.TextUtils;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.GridLayout;
import android.widget.LinearLayout;
import android.widget.ListView;
import android.widget.ProgressBar;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.io.File;
import java.io.IOException;
import java.text.SimpleDateFormat;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.Date;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import mobileapi.Listener;
import mobileapi.Mobileapi;
import mobileapi.Session;

public final class MainActivity extends Activity {
    private static final int REQUEST_INPUT = 1001;
    private static final int REQUEST_MANAGE_STORAGE = 1002;
    private static final int REQUEST_LEGACY_STORAGE = 1003;
    private static final String PROJECT_URL = "https://github.com/help660vip/LitePayloadDumper";
    private static final String PREFS = "settings";
    private static final String PREF_OUTPUT_PATH = "output_path";

    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final List<Partition> partitions = new ArrayList<>();
    private final Map<String, Boolean> checks = new HashMap<>();
    private final TextView[] infoValues = new TextView[8];

    private EditText inputEdit;
    private EditText searchEdit;
    private EditText threadEdit;
    private TextView storagePermissionValue;
    private TextView outputValue;
    private TextView partitionSummary;
    private TextView statusValue;
    private TextView logValue;
    private ProgressBar progress;
    private Button chooseInputButton;
    private Button readButton;
    private Button clearButton;
    private Button storagePermissionButton;
    private Button chooseOutputButton;
    private Button selectAllButton;
    private Button selectNoneButton;
    private Button extractButton;
    private Button cancelButton;
    private PartitionAdapter partitionAdapter;

    private Uri selectedInput;
    private String selectedInputName = "";
    private ParcelFileDescriptor inputDescriptor;
    private String outputPath = "";
    private volatile Session session;
    private volatile boolean busy;
    private volatile boolean cancelRequested;
    private boolean validatingOutput;
    private boolean waitingForStorageSettings;
    private boolean storageAccessAnnounced;
    private boolean restoredOutputValidated;
    private String searchQuery = "";

    private final Listener nativeListener = event -> {
        try {
            JSONObject json = new JSONObject(event);
        } catch (JSONException ignored) {
        }
        runOnUiThread(() -> handleNativeEvent(event));
    };

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        try {
            Mobileapi.setTempDir(new File(getCacheDir(), "go-temp").getAbsolutePath());
        } catch (Exception error) {
            Toast.makeText(this, error.getMessage(), Toast.LENGTH_LONG).show();
        }
        restoreOutputPath();
        buildInterface();
        updateStoragePermissionUi();
        getWindow().getDecorView().post(this::initializeStorageAccess);
    }

    private void buildInterface() {
        ScrollView scroll = new ScrollView(this);
        scroll.setFillViewport(true);
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setPadding(dp(16), dp(14), dp(16), dp(20));
        scroll.addView(content, new ScrollView.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));

        TextView title = text("LitePayloadDumper", 24, Color.rgb(25, 25, 25));
        title.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        content.addView(title);
        TextView subtitle = text("Android · 在线与本地固件分区提取", 13, Color.rgb(90, 90, 90));
        content.addView(subtitle, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 2, 0, 16));

        content.addView(section("固件来源"));
        LinearLayout inputRow = horizontal();
        inputEdit = new EditText(this);
        inputEdit.setSingleLine(true);
        inputEdit.setHint("粘贴在线地址，或选择本地固件");
        inputEdit.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        inputRow.addView(inputEdit, new LinearLayout.LayoutParams(0, dp(48), 1));
        chooseInputButton = button("选择文件");
        chooseInputButton.setOnClickListener(view -> chooseInput());
        inputRow.addView(chooseInputButton, margin(dp(96), dp(48), 8, 0, 0, 0));
        content.addView(inputRow);

        LinearLayout inputActions = horizontal();
        readButton = button("读取固件");
        readButton.setOnClickListener(view -> requestInspection());
        inputActions.addView(readButton, new LinearLayout.LayoutParams(0, dp(46), 1));
        clearButton = button("清空");
        clearButton.setOnClickListener(view -> clearInput());
        inputActions.addView(clearButton, margin(dp(84), dp(46), 8, 0, 0, 0));
        content.addView(inputActions, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 8, 0, 14));

        content.addView(section("固件信息"));
        GridLayout infoGrid = new GridLayout(this);
        infoGrid.setColumnCount(2);
        String[] labels = {"机型", "系统版本", "设备代号", "安卓版本", "补丁日期", "SDK", "包类型", "构建日期"};
        for (int index = 0; index < labels.length; index++) {
            TextView label = text(labels[index], 13, Color.rgb(75, 75, 75));
            label.setGravity(Gravity.CENTER_VERTICAL);
            infoGrid.addView(label, gridParams(0, dp(34), 0.30f));
            infoValues[index] = text("—", 13, Color.rgb(25, 25, 25));
            infoValues[index].setGravity(Gravity.CENTER_VERTICAL);
            infoValues[index].setTextIsSelectable(true);
            infoGrid.addView(infoValues[index], gridParams(0, dp(34), 0.70f));
        }
        content.addView(infoGrid, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 0, 0, 14));

        content.addView(section("分区"));
        searchEdit = new EditText(this);
        searchEdit.setSingleLine(true);
        searchEdit.setHint("按分区名称搜索");
        searchEdit.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence text, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence text, int start, int before, int count) {
                searchQuery = text.toString();
                refreshPartitions();
            }
            @Override public void afterTextChanged(Editable text) {}
        });
        content.addView(searchEdit, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(48)));

        LinearLayout selectionRow = horizontal();
        selectAllButton = button("全选");
        selectAllButton.setOnClickListener(view -> setAllChecks(true));
        selectNoneButton = button("全不选");
        selectNoneButton.setOnClickListener(view -> setAllChecks(false));
        selectionRow.addView(selectAllButton, new LinearLayout.LayoutParams(0, dp(44), 1));
        selectionRow.addView(selectNoneButton, margin(0, dp(44), 8, 0, 0, 0, 1));
        content.addView(selectionRow, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 6, 0, 0));

        partitionSummary = text("尚未读取固件", 12, Color.rgb(90, 90, 90));
        content.addView(partitionSummary, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 2, 7, 2, 4));
        ListView partitionList = new ListView(this);
        partitionList.setDividerHeight(1);
        partitionList.setNestedScrollingEnabled(true);
        partitionAdapter = new PartitionAdapter(this, checks);
        partitionList.setAdapter(partitionAdapter);
        content.addView(partitionList, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(320)));

        addExtractionSection(content);
        setContentView(scroll);
        setAllChecks(false);
    }

    private void addExtractionSection(LinearLayout content) {
        content.addView(section("保存与提取"), margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 16, 0, 0));
        LinearLayout permissionRow = horizontal();
        storagePermissionValue = text("正在检查存储权限…", 13, Color.rgb(60, 60, 60));
        storagePermissionValue.setGravity(Gravity.CENTER_VERTICAL);
        permissionRow.addView(storagePermissionValue, new LinearLayout.LayoutParams(0, dp(48), 1));
        storagePermissionButton = button("授予权限");
        storagePermissionButton.setOnClickListener(view -> requestStorageAccess());
        permissionRow.addView(storagePermissionButton, margin(dp(96), dp(48), 8, 0, 0, 0));
        content.addView(permissionRow);

        LinearLayout outputRow = horizontal();
        outputValue = text(outputPath.isEmpty() ? "尚未选择保存目录" : outputPath, 13, Color.rgb(60, 60, 60));
        outputValue.setGravity(Gravity.CENTER_VERTICAL);
        outputValue.setTextIsSelectable(true);
        outputRow.addView(outputValue, new LinearLayout.LayoutParams(0, dp(48), 1));
        chooseOutputButton = button("选择目录");
        chooseOutputButton.setOnClickListener(view -> chooseOutput());
        outputRow.addView(chooseOutputButton, margin(dp(96), dp(48), 8, 0, 0, 0));
        content.addView(outputRow);

        LinearLayout threadRow = horizontal();
        TextView threadLabel = text("线程数", 14, Color.rgb(45, 45, 45));
        threadLabel.setGravity(Gravity.CENTER_VERTICAL);
        threadRow.addView(threadLabel, new LinearLayout.LayoutParams(0, dp(48), 1));
        threadEdit = new EditText(this);
        threadEdit.setSingleLine(true);
        threadEdit.setText("4");
        threadEdit.setSelectAllOnFocus(true);
        threadEdit.setGravity(Gravity.CENTER);
        threadEdit.setInputType(InputType.TYPE_CLASS_NUMBER);
        threadRow.addView(threadEdit, new LinearLayout.LayoutParams(dp(88), dp(48)));
        TextView threadRange = text("1～64", 12, Color.rgb(90, 90, 90));
        threadRange.setGravity(Gravity.CENTER_VERTICAL);
        threadRow.addView(threadRange, margin(dp(54), dp(48), 8, 0, 0, 0));
        content.addView(threadRow, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 4, 0, 0));

        LinearLayout extractRow = horizontal();
        extractButton = button("提取所选分区");
        extractButton.setOnClickListener(view -> requestExtraction());
        cancelButton = button("取消");
        cancelButton.setEnabled(false);
        cancelButton.setOnClickListener(view -> cancelTask());
        extractRow.addView(extractButton, new LinearLayout.LayoutParams(0, dp(48), 1));
        extractRow.addView(cancelButton, margin(dp(88), dp(48), 8, 0, 0, 0));
        content.addView(extractRow, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 6, 0, 0));

        progress = new ProgressBar(this, null, android.R.attr.progressBarStyleHorizontal);
        progress.setMax(1000);
        content.addView(progress, margin(ViewGroup.LayoutParams.MATCH_PARENT, dp(12), 0, 10, 0, 0));
        statusValue = text("等待读取固件", 13, Color.rgb(45, 45, 45));
        content.addView(statusValue, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 7, 0, 0));

        content.addView(section("日志"), margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 16, 0, 0));
        logValue = text("", 12, Color.rgb(45, 45, 45));
        logValue.setTextIsSelectable(true);
        logValue.setPadding(dp(8), dp(6), dp(8), dp(6));
        logValue.setBackgroundColor(Color.rgb(245, 245, 245));
        content.addView(logValue, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(150)));

        TextView github = text("GitHub · " + PROJECT_URL, 14, Color.rgb(0, 88, 175));
        github.setGravity(Gravity.CENTER);
        github.setPadding(dp(4), dp(18), dp(4), dp(8));
        github.setOnClickListener(view -> openProject());
        content.addView(github, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
    }

    private void chooseInput() {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType("*/*");
        try {
            startActivityForResult(intent, REQUEST_INPUT);
        } catch (ActivityNotFoundException error) {
            showError("系统没有可用的文件选择器");
        }
    }

    private void chooseOutput() {
        if (busy || validatingOutput) {
            return;
        }
        if (!hasStorageAccess()) {
            showError("请先授予存储访问权限");
            requestStorageAccess();
            return;
        }
        try {
            File root = Environment.getExternalStorageDirectory().getCanonicalFile();
            File start = outputPath.isEmpty()
                    ? Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS)
                    : new File(outputPath);
            if (!start.exists() && !start.mkdirs()) {
                start = root;
            }
            showDirectoryPicker(StorageAccess.normalizeWithinRoot(root, start));
        } catch (Exception error) {
            showError("无法打开保存目录：" + message(error));
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQUEST_MANAGE_STORAGE) {
            waitingForStorageSettings = false;
            handleStorageAccessResult();
            return;
        }
        if (requestCode != REQUEST_INPUT || resultCode != RESULT_OK
                || data == null || data.getData() == null) {
            return;
        }
        Uri uri = data.getData();
        int flags = data.getFlags() & (Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
        if (flags != 0) {
            try {
                getContentResolver().takePersistableUriPermission(uri, flags);
            } catch (SecurityException ignored) {
            }
        }
        closeCurrentSession();
        selectedInput = uri;
        selectedInputName = displayName(uri);
        inputEdit.setText(selectedInputName);
        inputEdit.setSelection(inputEdit.length());
        appendLog("已选择本地固件：" + selectedInputName);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (storagePermissionValue == null) {
            return;
        }
        updateStoragePermissionUi();
        if (waitingForStorageSettings) {
            waitingForStorageSettings = false;
            handleStorageAccessResult();
        } else if (!hasStorageAccess()) {
            storageAccessAnnounced = false;
            restoredOutputValidated = false;
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == REQUEST_LEGACY_STORAGE) {
            handleStorageAccessResult();
        }
    }

    private void initializeStorageAccess() {
        if (hasStorageAccess()) {
            handleStorageAccessResult();
        } else {
            appendLog(Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
                    ? "需要授予“管理所有文件”权限后才能保存镜像"
                    : "需要授予存储读写权限后才能保存镜像");
            requestStorageAccess();
        }
    }

    private boolean hasStorageAccess() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            return Environment.isExternalStorageManager();
        }
        return checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
                && checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED;
    }

    private void updateStoragePermissionUi() {
        boolean granted = hasStorageAccess();
        if (storagePermissionValue != null) {
            storagePermissionValue.setText(granted
                    ? (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
                    ? "全部文件访问权限已授予" : "存储读写权限已授予")
                    : (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
                    ? "尚未授予全部文件访问权限" : "尚未授予存储读写权限"));
        }
        if (storagePermissionButton != null) {
            storagePermissionButton.setText(granted ? "已授权" : "授予权限");
            storagePermissionButton.setEnabled(!busy && !granted);
        }
        if (chooseOutputButton != null) {
            chooseOutputButton.setEnabled(!busy && !validatingOutput && granted);
        }
        if (extractButton != null) {
            extractButton.setEnabled(!busy && !validatingOutput && granted);
        }
    }

    private void requestStorageAccess() {
        if (busy) {
            return;
        }
        if (hasStorageAccess()) {
            handleStorageAccessResult();
            return;
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION,
                    Uri.parse("package:" + getPackageName()));
            waitingForStorageSettings = true;
            try {
                startActivityForResult(intent, REQUEST_MANAGE_STORAGE);
            } catch (ActivityNotFoundException error) {
                try {
                    startActivityForResult(
                            new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION),
                            REQUEST_MANAGE_STORAGE);
                } catch (ActivityNotFoundException fallbackError) {
                    waitingForStorageSettings = false;
                    showError("系统没有可用的全部文件访问权限设置页面");
                }
            }
        } else {
            requestPermissions(new String[]{
                    Manifest.permission.READ_EXTERNAL_STORAGE,
                    Manifest.permission.WRITE_EXTERNAL_STORAGE
            }, REQUEST_LEGACY_STORAGE);
        }
    }

    private void handleStorageAccessResult() {
        boolean granted = hasStorageAccess();
        updateStoragePermissionUi();
        if (!granted) {
            storageAccessAnnounced = false;
            restoredOutputValidated = false;
            appendLog("存储权限未授予，当前无法提取镜像");
            return;
        }
        if (!storageAccessAnnounced) {
            appendLog(Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
                    ? "全部文件访问权限已授予"
                    : "存储读写权限已授予");
            storageAccessAnnounced = true;
        }
        validateRestoredOutputPath();
    }

    private void showDirectoryPicker(File requested) {
        if (!hasStorageAccess()) {
            requestStorageAccess();
            return;
        }
        final File root;
        final File current;
        try {
            root = Environment.getExternalStorageDirectory().getCanonicalFile();
            current = StorageAccess.normalizeWithinRoot(root, requested);
        } catch (IOException error) {
            showError(message(error));
            return;
        }
        File[] children = current.listFiles(file -> file.isDirectory());
        if (children == null) {
            showError("无法读取目录：" + current.getAbsolutePath());
            return;
        }
        Arrays.sort(children, Comparator.comparing(File::getName, String.CASE_INSENSITIVE_ORDER));
        boolean hasParent = !current.equals(root);
        String[] labels = new String[children.length + (hasParent ? 1 : 0)];
        int offset = 0;
        if (hasParent) {
            labels[0] = "返回上级";
            offset = 1;
        }
        for (int index = 0; index < children.length; index++) {
            labels[index + offset] = children[index].getName();
        }
        int childOffset = offset;
        new AlertDialog.Builder(this)
                .setTitle("选择保存目录")
                .setMessage(current.getAbsolutePath())
                .setItems(labels, (dialog, which) -> {
                    if (hasParent && which == 0) {
                        showDirectoryPicker(current.getParentFile());
                    } else {
                        showDirectoryPicker(children[which - childOffset]);
                    }
                })
                .setNeutralButton("新建文件夹", (dialog, which) -> promptNewDirectory(root, current))
                .setNegativeButton("取消", null)
                .setPositiveButton("选择此目录",
                        (dialog, which) -> validateOutputPath(current, outputPath, true))
                .show();
    }

    private void promptNewDirectory(File root, File parent) {
        EditText name = new EditText(this);
        name.setSingleLine(true);
        name.setHint("文件夹名称");
        name.setSelectAllOnFocus(true);
        new AlertDialog.Builder(this)
                .setTitle("新建文件夹")
                .setView(name)
                .setNegativeButton("取消", (dialog, which) -> showDirectoryPicker(parent))
                .setPositiveButton("新建", (dialog, which) -> {
                    try {
                        File child = StorageAccess.createChildDirectory(root, parent, name.getText().toString());
                        appendLog("已新建文件夹：" + child.getAbsolutePath());
                        showDirectoryPicker(child);
                    } catch (Exception error) {
                        showError(message(error));
                        showDirectoryPicker(parent);
                    }
                })
                .show();
    }

    private void validateRestoredOutputPath() {
        if (!restoredOutputValidated && !outputPath.isEmpty() && hasStorageAccess()) {
            restoredOutputValidated = true;
            validateOutputPath(new File(outputPath), "", false);
        }
    }

    private void validateOutputPath(File candidate, String fallback, boolean fromPicker) {
        if (validatingOutput) {
            return;
        }
        validatingOutput = true;
        outputPath = fallback;
        chooseOutputButton.setEnabled(false);
        extractButton.setEnabled(false);
        outputValue.setText("正在验证目录写入权限…");
        appendLog("正在通过提取核心验证目录的创建、写入和删除权限…");
        worker.execute(() -> {
            Exception failure = null;
            File resolved = null;
            try {
                if (!hasStorageAccess()) {
                    throw new IOException("全部文件访问权限已经失效");
                }
                File root = Environment.getExternalStorageDirectory().getCanonicalFile();
                resolved = StorageAccess.normalizeWithinRoot(root, candidate);
                StorageAccess.verifyWritable(resolved);
            } catch (Exception error) {
                failure = error;
            }
            Exception result = failure;
            File verified = resolved;
            runOnUiThread(() -> {
                validatingOutput = false;
                if (result == null) {
                    outputPath = verified.getAbsolutePath();
                    getSharedPreferences(PREFS, MODE_PRIVATE).edit()
                            .putString(PREF_OUTPUT_PATH, outputPath)
                            .apply();
                    outputValue.setText(outputPath);
                    appendLog("保存目录已通过全部文件权限直接写入：" + outputPath);
                } else {
                    outputPath = fallback;
                    outputValue.setText(fallback.isEmpty() ? "尚未选择保存目录" : fallback);
                    if (fallback.isEmpty()) {
                        getSharedPreferences(PREFS, MODE_PRIVATE).edit().remove(PREF_OUTPUT_PATH).apply();
                    }
                    appendLog("保存目录不可写：" + message(result));
                    if (fromPicker || fallback.isEmpty()) {
                        String detail = "无法通过全部文件权限在所选目录创建镜像文件。\n\n"
                                + message(result);
                        showError(detail);
                    }
                }
                updateStoragePermissionUi();
            });
        });
    }

    private void requestInspection() {
        if (busy) {
            return;
        }
        String text = inputEdit.getText().toString().trim();
        boolean useLocal = selectedInput != null && text.equals(selectedInputName);
        if (!useLocal && !(text.startsWith("https://") || text.startsWith("http://"))) {
            showError("请选择本地固件，或输入 http/https 在线地址");
            return;
        }
        String lower = text.toLowerCase(Locale.ROOT);
        if (!useLocal && (lower.endsWith(".tgz") || lower.endsWith(".tar.gz"))) {
            new AlertDialog.Builder(this)
                    .setTitle("读取在线 TGZ")
                    .setMessage("TGZ 无法随机访问，需要先完整缓存一次。是否继续？")
                    .setNegativeButton("取消", null)
                    .setPositiveButton("继续", (dialog, which) -> inspectRemote(text))
                    .show();
            return;
        }
        if (useLocal) {
            inspectLocal();
        } else {
            inspectRemote(text);
        }
    }

    private void inspectLocal() {
        try {
            closeInputDescriptor();
            inputDescriptor = getContentResolver().openFileDescriptor(selectedInput, "r");
            if (inputDescriptor == null) {
                throw new IOException("无法打开所选文件");
            }
            Os.lseek(inputDescriptor.getFileDescriptor(), 0, OsConstants.SEEK_SET);
            inspect("/proc/self/fd/" + inputDescriptor.getFd(), selectedInputName);
        } catch (Exception error) {
            closeInputDescriptor();
            showError("该文件来源不支持随机读取，请先把固件保存到手机本地存储后再选择。\n\n" + message(error));
        }
    }

    private void inspectRemote(String address) {
        closeInputDescriptor();
        inspect(address, address);
    }

    private void inspect(String input, String label) {
        closeSessionOnly();
        Session current = Mobileapi.newSession(input, label);
        session = current;
        cancelRequested = false;
        setBusy(true);
        progress.setProgress(0);
        statusValue.setText("正在读取固件目录与元数据…");
        appendLog("开始分析：" + label);
        worker.execute(() -> {
            try {
                String details = current.inspect(nativeListener);
                runOnUiThread(() -> {
                    try {
                        applyDetails(new JSONObject(details));
                    } catch (JSONException error) {
                        showError("固件信息格式错误：" + error.getMessage());
                    }
                    setBusy(false);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    setBusy(false);
                    if (cancelRequested) {
                        statusValue.setText("读取任务已取消");
                        appendLog("读取任务已取消");
                    } else {
                        statusValue.setText("读取失败");
                        appendLog("读取失败：" + message(error));
                        showError(message(error));
                    }
                });
            }
        });
    }

    private void applyDetails(JSONObject details) throws JSONException {
        partitions.clear();
        checks.clear();
        JSONArray array = details.getJSONArray("partitions");
        for (int index = 0; index < array.length(); index++) {
            JSONObject item = array.getJSONObject(index);
            Partition partition = new Partition(
                    item.getString("name"), item.optLong("size"), item.optInt("operations"),
                    item.optBoolean("needsSource"), item.optString("unsupported"));
            partitions.add(partition);
            checks.put(partition.name, false);
        }
        JSONObject info = details.getJSONObject("info");
        String model = info.optString("model");
        String brand = info.optString("brand");
        if (!brand.isEmpty() && !model.toLowerCase(Locale.ROOT).contains(brand.toLowerCase(Locale.ROOT))) {
            model = brand + " " + model;
        }
        String packageType = info.optString("packageType");
        long payloadVersion = info.optLong("payloadVersion");
        if (payloadVersion > 0) {
            packageType += " / Payload v" + payloadVersion;
        }
        if (info.optBoolean("isDelta")) {
            packageType += "（增量）";
        }
        setInfo(0, model);
        setInfo(1, info.optString("systemVersion"));
        setInfo(2, info.optString("device"));
        setInfo(3, info.optString("android"));
        setInfo(4, info.optString("securityPatch"));
        setInfo(5, info.optString("sdk"));
        setInfo(6, packageType);
        setInfo(7, info.optString("buildDate"));
        refreshPartitions();
        long size = details.optLong("fileSize");
        statusValue.setText("读取完成：" + partitions.size() + " 个分区，固件大小 " + FormatUtils.bytes(size));
        progress.setProgress(1000);
        appendLog("读取完成：" + packageType + "，" + partitions.size() + " 个分区");
        if ("zip".equals(details.optString("mode"))) {
            appendLog("远程或本地 ZIP 不包含 payload.bin，已按通用 ZIP 模式读取");
            appendLog("ZIP 中共 " + details.optInt("archiveEntries") + " 个文件条目");
        }
        if (info.optBoolean("isDelta")) {
            appendLog("检测到增量 OTA；需要旧镜像的分区不支持提取");
        }
    }

    private void requestExtraction() {
        if (busy || session == null || partitions.isEmpty()) {
            showError("请先读取固件");
            return;
        }
        if (validatingOutput) {
            showError("正在验证保存目录，请稍候");
            return;
        }
        if (!hasStorageAccess()) {
            showError("存储权限已经失效，请重新授予后再提取");
            requestStorageAccess();
            return;
        }
        List<String> selected = selectedPartitions();
        if (selected.isEmpty()) {
            showError("请至少勾选一个要提取的分区");
            return;
        }
        if (outputPath.isEmpty()) {
            showError("请选择镜像保存目录");
            return;
        }
        final int threads;
        try {
            threads = FormatUtils.parseThreads(threadEdit.getText().toString());
        } catch (IllegalArgumentException error) {
            showError(error.getMessage());
            threadEdit.requestFocus();
            threadEdit.selectAll();
            return;
        }
        new AlertDialog.Builder(this)
                .setTitle("开始提取")
                .setMessage("将提取 " + selected.size() + " 个分区。同名镜像会被覆盖，是否继续？")
                .setNegativeButton("取消", null)
                .setPositiveButton("继续", (dialog, which) -> extract(selected, threads))
                .show();
    }

    private void extract(List<String> selected, int threads) {
        cancelRequested = false;
        setBusy(true);
        progress.setProgress(0);
        statusValue.setText("正在准备保存文件…");
        appendLog("开始提取：" + TextUtils.join(", ", selected) + "；线程数：" + threads);
        Session current = session;
        File destination = new File(outputPath);
        worker.execute(() -> {
            File staging = null;
            boolean success = false;
            try {
                staging = StorageAccess.createStagingDirectory(destination);
                JSONArray names = new JSONArray();
                for (String name : selected) {
                    names.put(name);
                }
                current.extract(staging.getAbsolutePath(), names.toString(), threads, nativeListener);
                StorageAccess.commitStaging(destination, staging, selected);
                staging = null;
                success = true;
                runOnUiThread(() -> {
                    progress.setProgress(1000);
                    statusValue.setText("提取完成，镜像已保存到 " + destination.getAbsolutePath());
                    appendLog("全部完成，镜像已直接写入：" + destination.getAbsolutePath());
                    Toast.makeText(this, "所选分区已提取完成", Toast.LENGTH_LONG).show();
                });
            } catch (Exception error) {
                StorageAccess.discardStaging(destination, staging);
                runOnUiThread(() -> {
                    if (cancelRequested) {
                        statusValue.setText("任务已取消，未完成镜像已清理");
                        appendLog("任务已取消，未完成镜像已清理");
                    } else {
                        statusValue.setText("提取失败：" + message(error));
                        appendLog("提取失败：" + message(error));
                        showError(message(error));
                    }
                });
            } finally {
                boolean completed = success;
                runOnUiThread(() -> {
                    setBusy(false);
                    if (!completed && progress.getProgress() >= 1000) {
                        progress.setProgress(0);
                    }
                });
            }
        });
    }

    private void cancelTask() {
        if (!busy || session == null) {
            return;
        }
        cancelRequested = true;
        session.cancel();
        cancelButton.setEnabled(false);
        statusValue.setText("正在取消并清理未完成镜像…");
        appendLog("收到取消请求，正在安全停止…");
    }

    private void handleNativeEvent(String event) {
        try {
            JSONObject json = new JSONObject(event);
            String type = json.optString("type");
            double percent = json.optDouble("percent", 0);
            if (percent >= 0) {
                progress.setProgress((int) Math.min(1000, Math.round(percent * 10)));
            }
            if ("inspection".equals(type)) {
                String stage = json.optString("stage", "读取");
                long done = json.optLong("done");
                long total = json.optLong("total");
                statusValue.setText(total > 0
                        ? stage + "：" + FormatUtils.bytes(done) + " / " + FormatUtils.bytes(total)
                        : stage + "：已处理 " + FormatUtils.bytes(done));
            } else if ("extraction".equals(type)) {
                String stage = json.optString("stage", "提取");
                String partition = json.optString("partition");
                long done = json.optLong("bytesDone");
                long total = json.optLong("bytesTotal");
                statusValue.setText(stage + " " + partition + "：" + FormatUtils.bytes(done)
                        + (total > 0 ? " / " + FormatUtils.bytes(total) : "")
                        + String.format(Locale.US, "（%.1f%%）", percent));
                if (json.optBoolean("partitionDone")) {
                    String failure = json.optString("error");
                    appendLog(partition + ".img - " + (failure.isEmpty() ? "OK" : "失败：" + failure));
                }
            }
        } catch (JSONException ignored) {
        }
    }

    private List<String> selectedPartitions() {
        List<String> selected = new ArrayList<>();
        for (Partition partition : partitions) {
            if (partition.extractable() && Boolean.TRUE.equals(checks.get(partition.name))) {
                selected.add(partition.name);
            }
        }
        return selected;
    }

    private void setAllChecks(boolean checked) {
        for (Partition partition : partitions) {
            checks.put(partition.name, checked && partition.extractable());
        }
        refreshPartitions();
    }

    private void refreshPartitions() {
        if (partitionAdapter == null) {
            return;
        }
        partitionAdapter.setItems(partitions, searchQuery);
        int visible = 0;
        for (Partition partition : partitions) {
            if (FormatUtils.matches(partition.name, searchQuery)) {
                visible++;
            }
        }
        partitionSummary.setText(partitions.isEmpty() ? "尚未读取固件"
                : "共 " + partitions.size() + " 个分区，当前显示 " + visible + " 个；默认不勾选");
    }

    private void setBusy(boolean value) {
        busy = value;
        inputEdit.setEnabled(!value);
        chooseInputButton.setEnabled(!value);
        readButton.setEnabled(!value);
        clearButton.setEnabled(!value);
        searchEdit.setEnabled(!value);
        storagePermissionButton.setEnabled(!value && !hasStorageAccess());
        chooseOutputButton.setEnabled(!value && !validatingOutput && hasStorageAccess());
        selectAllButton.setEnabled(!value);
        selectNoneButton.setEnabled(!value);
        threadEdit.setEnabled(!value);
        extractButton.setEnabled(!value && !validatingOutput && hasStorageAccess());
        cancelButton.setEnabled(value);
        partitionAdapter.setEnabled(!value);
        if (value) {
            getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        } else {
            getWindow().clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        }
    }

    private void clearInput() {
        if (busy) {
            return;
        }
        closeCurrentSession();
        selectedInput = null;
        selectedInputName = "";
        inputEdit.setText("");
        partitions.clear();
        checks.clear();
        setInfoBlank();
        refreshPartitions();
        progress.setProgress(0);
        statusValue.setText("等待读取固件");
        logValue.setText("");
    }

    private void restoreOutputPath() {
        SharedPreferences prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        outputPath = prefs.getString(PREF_OUTPUT_PATH, "").trim();
    }

    private void closeCurrentSession() {
        closeSessionOnly();
        closeInputDescriptor();
    }

    private void closeSessionOnly() {
        Session current = session;
        session = null;
        if (current != null) {
            current.close();
        }
    }

    private void closeInputDescriptor() {
        if (inputDescriptor != null) {
            try {
                inputDescriptor.close();
            } catch (IOException ignored) {
            }
            inputDescriptor = null;
        }
    }

    @Override
    protected void onDestroy() {
        cancelRequested = true;
        closeCurrentSession();
        worker.shutdownNow();
        super.onDestroy();
    }

    private void openProject() {
        try {
            startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(PROJECT_URL)));
        } catch (ActivityNotFoundException error) {
            showError(PROJECT_URL);
        }
    }

    private String displayName(Uri uri) {
        String name = "";
        try (Cursor cursor = getContentResolver().query(uri, new String[]{OpenableColumns.DISPLAY_NAME}, null, null, null)) {
            if (cursor != null && cursor.moveToFirst()) {
                name = cursor.getString(0);
            }
        } catch (Exception ignored) {
        }
        if (name == null || name.trim().isEmpty()) {
            name = uri.getLastPathSegment();
        }
        if (name == null || name.trim().isEmpty()) {
            name = uri.toString();
        }
        return name;
    }

    private void setInfo(int index, String value) {
        infoValues[index].setText(value == null || value.trim().isEmpty() ? "—" : value);
    }

    private void setInfoBlank() {
        for (TextView value : infoValues) {
            if (value != null) {
                value.setText("—");
            }
        }
    }

    private void appendLog(String line) {
        if (line == null || line.isEmpty() || logValue == null) {
            return;
        }
        String time = new SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(new Date());
        logValue.append("[" + time + "] " + line + "\n");
    }

    private void showError(String message) {
        new AlertDialog.Builder(this).setTitle("LitePayloadDumper").setMessage(message).setPositiveButton("确定", null).show();
    }

    private static String message(Throwable error) {
        String value = error.getMessage();
        return value == null || value.trim().isEmpty() ? error.getClass().getSimpleName() : value;
    }

    private TextView section(String value) {
        TextView view = text(value, 16, Color.rgb(25, 25, 25));
        view.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        view.setPadding(0, dp(4), 0, dp(5));
        return view;
    }

    private TextView text(String value, float size, int color) {
        TextView view = new TextView(this);
        view.setText(value);
        view.setTextSize(size);
        view.setTextColor(color);
        return view;
    }

    private Button button(String value) {
        Button button = new Button(this);
        button.setText(value);
        button.setTextSize(13);
        button.setAllCaps(false);
        return button;
    }

    private LinearLayout horizontal() {
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.HORIZONTAL);
        layout.setGravity(Gravity.CENTER_VERTICAL);
        return layout;
    }

    private GridLayout.LayoutParams gridParams(int width, int height, float weight) {
        GridLayout.LayoutParams params = new GridLayout.LayoutParams();
        params.width = width;
        params.height = height;
        params.columnSpec = GridLayout.spec(GridLayout.UNDEFINED, weight);
        return params;
    }

    private LinearLayout.LayoutParams margin(int width, int height, int left, int top, int right, int bottom) {
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(width, height);
        params.setMargins(dp(left), dp(top), dp(right), dp(bottom));
        return params;
    }

    private LinearLayout.LayoutParams margin(int width, int height, int left, int top, int right, int bottom, float weight) {
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(width, height, weight);
        params.setMargins(dp(left), dp(top), dp(right), dp(bottom));
        return params;
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }
}
