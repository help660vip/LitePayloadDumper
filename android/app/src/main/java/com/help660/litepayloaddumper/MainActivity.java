package com.help660.litepayloaddumper;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.UriPermission;
import android.database.Cursor;
import android.graphics.Color;
import android.graphics.Typeface;
import android.net.Uri;
import android.os.Bundle;
import android.os.ParcelFileDescriptor;
import android.provider.DocumentsContract;
import android.provider.OpenableColumns;
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
import java.util.Collections;
import java.util.Date;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import mobileapi.Listener;
import mobileapi.Mobileapi;
import mobileapi.Session;

public final class MainActivity extends Activity {
    private static final int REQUEST_INPUT = 1001;
    private static final int REQUEST_OUTPUT = 1002;
    private static final String PROJECT_URL = "https://github.com/help660vip/LitePayloadDumper";
    private static final String PREFS = "settings";
    private static final String PREF_OUTPUT_TREE = "output_tree";

    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final List<Partition> partitions = new ArrayList<>();
    private final Map<String, Boolean> checks = new HashMap<>();
    private final Set<String> completedPartitions = Collections.synchronizedSet(new HashSet<>());
    private final TextView[] infoValues = new TextView[8];

    private EditText inputEdit;
    private EditText searchEdit;
    private EditText threadEdit;
    private TextView outputValue;
    private TextView partitionSummary;
    private TextView statusValue;
    private TextView logValue;
    private ProgressBar progress;
    private Button chooseInputButton;
    private Button readButton;
    private Button clearButton;
    private Button chooseOutputButton;
    private Button selectAllButton;
    private Button selectNoneButton;
    private Button extractButton;
    private Button cancelButton;
    private PartitionAdapter partitionAdapter;

    private Uri selectedInput;
    private String selectedInputName = "";
    private ParcelFileDescriptor inputDescriptor;
    private Uri outputTree;
    private volatile Session session;
    private volatile SafOutputBridge activeBridge;
    private volatile boolean busy;
    private volatile boolean cancelRequested;
    private boolean validatingOutput;
    private String searchQuery = "";

    private final Listener nativeListener = event -> {
        try {
            JSONObject json = new JSONObject(event);
            if ("extraction".equals(json.optString("type"))
                    && json.optBoolean("partitionDone")
                    && json.optString("error").isEmpty()) {
                completedPartitions.add(json.optString("partition"));
            }
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
        restoreOutputTree();
        buildInterface();
        validateRestoredOutputTree();
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
        LinearLayout outputRow = horizontal();
        outputValue = text(outputTree == null ? "尚未选择保存目录" : displayName(outputTree), 13, Color.rgb(60, 60, 60));
        outputValue.setGravity(Gravity.CENTER_VERTICAL);
        outputValue.setTextIsSelectable(true);
        outputRow.addView(outputValue, new LinearLayout.LayoutParams(0, dp(48), 1));
        chooseOutputButton = button("授权目录");
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
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
                | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION | Intent.FLAG_GRANT_PREFIX_URI_PERMISSION);
        try {
            startActivityForResult(intent, REQUEST_OUTPUT);
        } catch (ActivityNotFoundException error) {
            showError("系统没有可用的目录授权界面");
        }
    }

    @Override
    @SuppressLint("WrongConstant")
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (resultCode != RESULT_OK || data == null || data.getData() == null) {
            return;
        }
        Uri uri = data.getData();
        int flags = data.getFlags() & (Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
        if (requestCode == REQUEST_INPUT && flags != 0) {
            try {
                getContentResolver().takePersistableUriPermission(uri, flags);
            } catch (SecurityException ignored) {
            }
        }
        if (requestCode == REQUEST_INPUT) {
            closeCurrentSession();
            selectedInput = uri;
            selectedInputName = displayName(uri);
            inputEdit.setText(selectedInputName);
            inputEdit.setSelection(inputEdit.length());
            appendLog("已选择本地固件：" + selectedInputName);
        } else if (requestCode == REQUEST_OUTPUT) {
            if ((flags & Intent.FLAG_GRANT_WRITE_URI_PERMISSION) == 0) {
                showError("系统没有授予目录写入权限，请重新选择并点击“使用此文件夹”或“允许”");
                return;
            }
            boolean persisted = false;
            if ((data.getFlags() & Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION) != 0) {
                try {
                    getContentResolver().takePersistableUriPermission(uri, flags);
                    persisted = hasPersistedWritePermission(uri);
                } catch (SecurityException ignored) {
                    // Some third-party document providers only grant access for the current app session.
                }
            }
            validateOutputTree(uri, persisted, outputTree, true);
        }
    }

    private void validateRestoredOutputTree() {
        Uri restored = outputTree;
        if (restored != null) {
            validateOutputTree(restored, true, null, false);
        }
    }

    private void validateOutputTree(Uri candidate, boolean persisted, Uri fallback, boolean fromPicker) {
        if (validatingOutput) {
            return;
        }
        validatingOutput = true;
        outputTree = fallback;
        chooseOutputButton.setEnabled(false);
        extractButton.setEnabled(false);
        outputValue.setText("正在验证目录写入权限…");
        appendLog("正在验证保存目录的创建、写入和删除权限…");
        worker.execute(() -> {
            Exception failure = null;
            try {
                SafOutputBridge.verifyWritable(this, candidate);
            } catch (Exception error) {
                failure = error;
            }
            Exception result = failure;
            runOnUiThread(() -> {
                validatingOutput = false;
                chooseOutputButton.setEnabled(!busy);
                extractButton.setEnabled(!busy);
                if (result == null) {
                    outputTree = candidate;
                    SharedPreferences.Editor editor = getSharedPreferences(PREFS, MODE_PRIVATE).edit();
                    if (persisted) {
                        editor.putString(PREF_OUTPUT_TREE, candidate.toString());
                    } else {
                        editor.remove(PREF_OUTPUT_TREE);
                    }
                    editor.apply();
                    outputValue.setText(displayName(candidate));
                    appendLog("保存目录已获得写入权限：" + outputValue.getText());
                    if (!persisted) {
                        appendLog("当前文件管理器只提供本次授权，应用重启后需要重新选择保存目录");
                    }
                } else {
                    outputTree = fallback;
                    outputValue.setText(fallback == null ? "尚未选择保存目录" : displayName(fallback));
                    if (fallback == null) {
                        getSharedPreferences(PREFS, MODE_PRIVATE).edit().remove(PREF_OUTPUT_TREE).apply();
                    }
                    appendLog("保存目录不可写：" + message(result));
                    String detail = "无法在所选目录创建镜像文件，请重新选择并在系统页面点击“使用此文件夹”或“允许”。\n\n"
                            + message(result);
                    if (fromPicker || fallback == null) {
                        showError(detail);
                    }
                }
            });
        });
    }

    private boolean hasPersistedWritePermission(Uri uri) {
        for (UriPermission permission : getContentResolver().getPersistedUriPermissions()) {
            if (uri.equals(permission.getUri()) && permission.isWritePermission()) {
                return true;
            }
        }
        return false;
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
        List<String> selected = selectedPartitions();
        if (selected.isEmpty()) {
            showError("请至少勾选一个要提取的分区");
            return;
        }
        if (outputTree == null) {
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
        completedPartitions.clear();
        setBusy(true);
        progress.setProgress(0);
        statusValue.setText("正在准备保存文件…");
        appendLog("开始提取：" + TextUtils.join(", ", selected) + "；线程数：" + threads);
        Session current = session;
        worker.execute(() -> {
            SafOutputBridge bridge = null;
            boolean success = false;
            try {
                bridge = SafOutputBridge.create(this, outputTree, selected);
                activeBridge = bridge;
                JSONArray names = new JSONArray();
                for (String name : selected) {
                    names.put(name);
                }
                current.extract(bridge.path(), names.toString(), threads, nativeListener);
                success = true;
                runOnUiThread(() -> {
                    progress.setProgress(1000);
                    statusValue.setText("提取完成，镜像已保存到所选目录");
                    appendLog("全部完成，镜像已写入所选目录");
                    Toast.makeText(this, "所选分区已提取完成", Toast.LENGTH_LONG).show();
                });
            } catch (Exception error) {
                if (bridge != null) {
                    bridge.deleteIncomplete(new HashSet<>(completedPartitions));
                }
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
                activeBridge = null;
                if (bridge != null) {
                    bridge.close();
                }
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
        chooseOutputButton.setEnabled(!value && !validatingOutput);
        selectAllButton.setEnabled(!value);
        selectNoneButton.setEnabled(!value);
        threadEdit.setEnabled(!value);
        extractButton.setEnabled(!value && !validatingOutput);
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

    private void restoreOutputTree() {
        SharedPreferences prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        String saved = prefs.getString(PREF_OUTPUT_TREE, "");
        if (!saved.isEmpty()) {
            Uri restored = Uri.parse(saved);
            if (hasPersistedWritePermission(restored)) {
                outputTree = restored;
            } else {
                prefs.edit().remove(PREF_OUTPUT_TREE).apply();
            }
        }
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
        SafOutputBridge bridge = activeBridge;
        if (bridge != null) {
            bridge.close();
        }
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
            try {
                name = DocumentsContract.getTreeDocumentId(uri);
            } catch (Exception ignored) {
                name = uri.toString();
            }
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
