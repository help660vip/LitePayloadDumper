package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (app *application) handleCommand(id, notification int) {
	if id == idSearch && notification == enChange {
		if !app.busy {
			app.syncVisibleChecks()
			app.populatePartitions()
		}
		return
	}
	if notification != bnClicked {
		return
	}
	switch id {
	case idGitHub:
		if !openExternalURL(app.hwnd, projectURL) {
			showMessage(app.hwnd, "无法打开浏览器", "请手动访问：\n"+projectURL, mbOK|mbIconError)
		}
	case idBrowse:
		if !app.busy {
			if filename := chooseInputFile(app.hwnd); filename != "" {
				app.loadFile(filename)
			}
		}
	case idLoad:
		if !app.busy {
			if input := strings.TrimSpace(getText(app.inputEdit)); input != "" {
				app.loadFile(input)
			} else {
				showMessage(app.hwnd, "请选择固件", "请选择本地固件，或粘贴在线地址。", mbOK|mbIconInfo)
			}
		}
	case idSelectAll:
		app.setAllChecks(true)
	case idSelectNone:
		app.setAllChecks(false)
	case idOutputBrowse:
		if folder := chooseFolder(app.hwnd, "选择分区镜像保存目录"); folder != "" {
			setText(app.outputEdit, folder)
		}
	case idExtract:
		app.startExtraction()
	case idCancel:
		app.cancelExtraction()
	}
}

func (app *application) loadFile(filename string) {
	filename = strings.TrimSpace(filename)
	if isRemoteInput(filename) && isTGZInput(filename) {
		warning := "在线 TGZ 需要先完整缓存到临时目录。文件只下载一次，取消或退出时自动清理。是否继续？"
		if showMessage(app.hwnd, "读取在线 TGZ", warning, mbYesNo|mbIconQuestion) != idYes {
			return
		}
	}
	if !isRemoteInput(filename) {
		absolute, err := filepath.Abs(filename)
		if err != nil {
			showMessage(app.hwnd, "路径无效", err.Error(), mbOK|mbIconError)
			return
		}
		filename = absolute
	}
	app.loadSerial++
	serial := app.loadSerial
	setText(app.inputEdit, filename)
	setText(app.outputEdit, defaultOutputDir(filename))
	if app.details != nil {
		app.details.Cleanup()
	}
	app.details = nil
	app.partitions = nil
	app.visiblePartitions = nil
	app.partitionChecks = make(map[string]bool)
	setText(app.searchEdit, "")
	sendMessage(app.partitionList, lvmDeleteAllItems, 0, 0)
	app.setInfo(DeviceInfo{})
	setText(app.logEdit, "")
	setText(app.statusLabel, "正在读取固件目录与元数据...")
	sendMessage(app.progressBar, pbmSetPos, 0, 0)
	app.appendLog("开始分析：" + filename)
	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel
	app.setWorking(true, true)
	var progressMu sync.Mutex
	lastProgress := time.Time{}
	inspectionProgress := func(progress InspectionProgress) {
		progressMu.Lock()
		now := time.Now()
		shouldPost := lastProgress.IsZero() || now.Sub(lastProgress) >= 120*time.Millisecond || (progress.Total > 0 && progress.Done >= progress.Total)
		if shouldPost {
			lastProgress = now
		}
		progressMu.Unlock()
		if !shouldPost {
			return
		}
		app.queueUI(func() {
			position := 0
			if progress.Total > 0 {
				position = int(progress.Done * 1000 / progress.Total)
			}
			sendMessage(app.progressBar, pbmSetPos, uintptr(position), 0)
			if progress.Total > 0 {
				setText(app.statusLabel, fmt.Sprintf("%s：%s / %s（%.1f%%）", progress.Stage, formatBytes(progress.Done), formatBytes(progress.Total), float64(position)/10))
			} else {
				setText(app.statusLabel, fmt.Sprintf("%s：已处理 %s", progress.Stage, formatBytes(progress.Done)))
			}
		})
	}

	go func() {
		details, err := inspectPackageContext(ctx, filename, inspectionProgress)
		app.queueUI(func() {
			if serial != app.loadSerial {
				return
			}
			app.cancel = nil
			app.setWorking(false, false)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					setText(app.statusLabel, "读取任务已取消，临时缓存已清理。")
					app.appendLog("读取任务已取消，临时缓存已清理。")
					if app.closeAfterCancel {
						procDestroyWindow.Call(app.hwnd)
					}
					return
				}
				setText(app.statusLabel, "读取失败。")
				app.appendLog("读取失败：" + err.Error())
				showMessage(app.hwnd, "读取失败", err.Error(), mbOK|mbIconError)
				return
			}
			app.details = details
			app.partitions = details.Partitions
			app.populatePartitions()
			app.setInfo(details.Info)
			if details.Mode == packageModeZIP {
				location := "ZIP"
				if details.Remote {
					location = "远程 ZIP"
				}
				app.appendLog("检测到" + location + " 不包含 payload.bin，直接按通用 ZIP 模式读取。")
				app.appendLog(fmt.Sprintf("ZIP 中共 %d 个文件条目，识别到 %d 个镜像分区。", details.ArchiveEntries, len(details.Partitions)))
			} else if details.Mode == packageModeTGZ {
				app.appendLog(fmt.Sprintf("TGZ 中共 %d 个文件条目，识别到 %d 个镜像分区。", details.ArchiveEntries, len(details.Partitions)))
				if details.Remote {
					app.appendLog("远程 TGZ 已完整缓存一次；提取将复用临时缓存，退出时自动删除。")
				}
			} else if details.Remote {
				app.appendLog("在线按需模式：只读取 ZIP 目录、Payload 清单和所选分区对应的字节段。")
			}
			setText(app.statusLabel, fmt.Sprintf("读取完成：共 %d 个分区，固件大小 %s。", len(details.Partitions), formatBytes(uint64(details.FileSize))))
			if details.Mode == packageModePayload {
				app.appendLog(fmt.Sprintf("读取完成：Payload v%d，%d 个分区。", details.Info.PayloadVersion, len(details.Partitions)))
			} else {
				app.appendLog(fmt.Sprintf("读取完成：%s，%d 个分区。", details.Info.PackageType, len(details.Partitions)))
			}
			if details.Info.Fingerprint != "" {
				app.appendLog("构建指纹：" + details.Info.Fingerprint)
			}
			if details.Info.IsDelta {
				app.appendLog("检测到增量 OTA；需要旧镜像的分区不支持提取，请使用完整固件。")
			}
			if app.closeAfterCancel {
				procDestroyWindow.Call(app.hwnd)
			}
		})
	}()
}

func defaultOutputDir(input string) string {
	base := "payload"
	parent := ""
	if isRemoteInput(input) {
		if parsed, err := url.Parse(input); err == nil {
			name := urlpath.Base(parsed.Path)
			base = name
			if strings.HasSuffix(strings.ToLower(base), ".tar.gz") {
				base = base[:len(base)-len(".tar.gz")]
			} else {
				base = strings.TrimSuffix(base, urlpath.Ext(base))
			}
		}
		parent = defaultRemoteOutputParent()
	} else {
		base = filepath.Base(input)
		if strings.HasSuffix(strings.ToLower(base), ".tar.gz") {
			base = base[:len(base)-len(".tar.gz")]
		} else {
			base = strings.TrimSuffix(base, filepath.Ext(base))
		}
		parent = filepath.Dir(input)
	}
	if strings.TrimSpace(base) == "" || base == "." || base == "/" {
		base = "payload"
	}
	return filepath.Join(parent, base+"_提取")
}

func defaultRemoteOutputParent() string {
	downloads := knownDownloadsDirectory()
	downloadsUsable := false
	if downloads != "" {
		if stat, err := os.Stat(downloads); err == nil && stat.IsDir() {
			downloadsUsable = true
		}
	}

	executableDir := ""
	if executable, err := os.Executable(); err == nil {
		executableDir = filepath.Dir(executable)
	}
	if parent := chooseRemoteOutputParent(downloads, executableDir, downloadsUsable); parent != "" {
		return parent
	}
	if currentDir, err := os.Getwd(); err == nil {
		return currentDir
	}
	return "."
}

func chooseRemoteOutputParent(downloads, executableDir string, downloadsUsable bool) string {
	if downloadsUsable && downloads != "" && !strings.EqualFold(filepath.VolumeName(filepath.Clean(downloads)), "C:") {
		return downloads
	}
	return executableDir
}

func (app *application) populatePartitions() {
	sendMessage(app.partitionList, lvmDeleteAllItems, 0, 0)
	app.visiblePartitions = filterPartitions(app.partitions, getText(app.searchEdit))
	for row, part := range app.visiblePartitions {
		status := "可提取"
		if len(part.UnsupportedOps) > 0 {
			status = "暂不支持：" + strings.Join(part.UnsupportedOps, ", ")
		} else if part.NeedsSource {
			status = "增量分区（不支持）"
		}
		addListItem(app.partitionList, row, []string{part.Name, formatBytes(part.Size), fmt.Sprint(part.Operations), status}, app.partitionChecks[part.Name])
	}
}

func filterPartitions(partitions []PartitionItem, query string) []PartitionItem {
	query = strings.ToLower(strings.TrimSpace(query))
	visible := make([]PartitionItem, 0, len(partitions))
	for _, part := range partitions {
		if query == "" || strings.Contains(strings.ToLower(part.Name), query) {
			visible = append(visible, part)
		}
	}
	return visible
}

func (app *application) syncVisibleChecks() {
	if app.partitionChecks == nil {
		app.partitionChecks = make(map[string]bool)
	}
	rows := listItemCount(app.partitionList)
	if rows > len(app.visiblePartitions) {
		rows = len(app.visiblePartitions)
	}
	for row := 0; row < rows; row++ {
		app.partitionChecks[app.visiblePartitions[row].Name] = listItemChecked(app.partitionList, row)
	}
}

func (app *application) setInfo(info DeviceInfo) {
	model := info.Model
	if info.Brand != "" && model != "" && !strings.Contains(strings.ToLower(model), strings.ToLower(info.Brand)) {
		model = info.Brand + " " + model
	}
	typeText := info.PackageType
	if info.PayloadVersion > 0 {
		typeText += fmt.Sprintf(" / Payload v%d", info.PayloadVersion)
	}
	if info.IsDelta {
		typeText += "（增量）"
	}
	values := []string{model, info.SystemVersion, info.Device, info.Android, info.SecurityPatch, info.SDK, typeText, info.BuildDate}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			value = "—"
		}
		setText(app.infoValues[i], value)
	}
}

func (app *application) setAllChecks(checked bool) {
	if app.busy {
		return
	}
	app.syncVisibleChecks()
	for _, part := range app.partitions {
		app.partitionChecks[part.Name] = checked
	}
	for row, part := range app.visiblePartitions {
		setListChecked(app.partitionList, row, checked)
		app.partitionChecks[part.Name] = checked
	}
}

func (app *application) selectedPartitions() ([]string, []PartitionItem) {
	app.syncVisibleChecks()
	var names []string
	var items []PartitionItem
	for _, part := range app.partitions {
		if app.partitionChecks[part.Name] {
			names = append(names, part.Name)
			items = append(items, part)
		}
	}
	return names, items
}

func (app *application) selectedThreads() (int, error) {
	return parseThreadCount(getText(app.threadEdit))
}

func parseThreadCount(text string) (int, error) {
	threads, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || threads < 1 || threads > 64 {
		return 0, fmt.Errorf("线程数必须是 1～64 的整数")
	}
	return threads, nil
}

func (app *application) startExtraction() {
	if app.busy || app.details == nil {
		showMessage(app.hwnd, "尚未读取固件", "请先选择 OTA、线刷 ZIP、TGZ 或 payload.bin。", mbOK|mbIconInfo)
		return
	}
	names, items := app.selectedPartitions()
	if len(names) == 0 {
		showMessage(app.hwnd, "未选择分区", "请至少勾选一个要提取的分区。", mbOK|mbIconInfo)
		return
	}
	for _, item := range items {
		if len(item.UnsupportedOps) > 0 {
			showMessage(app.hwnd, "暂不支持", item.Name+" 包含当前内核不支持的操作："+strings.Join(item.UnsupportedOps, ", "), mbOK|mbIconError)
			return
		}
		if item.NeedsSource {
			showMessage(app.hwnd, "不支持增量分区", item.Name+" 需要旧版本镜像。本工具仅提取完整固件，请改用对应的完整包。", mbOK|mbIconInfo)
			return
		}
	}
	threads, threadErr := app.selectedThreads()
	if threadErr != nil {
		showMessage(app.hwnd, "线程数无效", threadErr.Error(), mbOK|mbIconInfo)
		procSetFocus.Call(app.threadEdit)
		return
	}
	outputDir := strings.TrimSpace(getText(app.outputEdit))
	if outputDir == "" {
		showMessage(app.hwnd, "未选择目录", "请选择分区镜像保存目录。", mbOK|mbIconInfo)
		return
	}
	overwriteCount := 0
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(outputDir, name+".img")); err == nil {
			overwriteCount++
		}
	}
	if overwriteCount > 0 {
		text := fmt.Sprintf("保存目录中已有 %d 个同名镜像。是否覆盖？", overwriteCount)
		if showMessage(app.hwnd, "确认覆盖", text, mbYesNo|mbIconQuestion) != idYes {
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel
	app.setWorking(true, true)
	sendMessage(app.progressBar, pbmSetPos, 0, 0)
	if app.details.Mode == packageModeTGZ {
		setText(app.statusLabel, "正在扫描 TGZ 缓存并提取所选镜像，可随时取消...")
	} else if app.details.Mode == packageModeZIP && app.details.Remote {
		setText(app.statusLabel, "在线按需连接中：正在读取 ZIP 目录与所选镜像，可随时取消...")
	} else if app.details.Remote {
		setText(app.statusLabel, "在线按需连接中：正在读取 Payload 数据，可随时取消...")
	} else {
		setText(app.statusLabel, fmt.Sprintf("正在提取 %d 个分区（%d 线程）...", len(names), threads))
	}
	app.appendLog(fmt.Sprintf("开始提取：%s；线程数：%d。", strings.Join(names, ", "), threads))
	if app.details.Mode == packageModeTGZ && threads > 1 {
		app.appendLog("TGZ 是单一串流，将按归档顺序扫描提取；线程设置对 TGZ 不生效。")
	}

	var callbackMu sync.Mutex
	lastUpdate := time.Time{}
	progressCallback := func(progress ExtractionProgress) {
		callbackMu.Lock()
		now := time.Now()
		shouldPost := progress.PartitionDone || now.Sub(lastUpdate) >= 120*time.Millisecond
		if shouldPost {
			lastUpdate = now
		}
		callbackMu.Unlock()
		if !shouldPost {
			return
		}
		update := progress
		app.queueUI(func() { app.showProgress(update) })
	}

	go func() {
		err := extractPackageWithItems(ctx, app.details.Path, outputDir, "", names, items, threads, progressCallback)
		app.queueUI(func() {
			app.cancel = nil
			app.setWorking(false, false)
			if errors.Is(err, context.Canceled) {
				setText(app.statusLabel, "任务已取消；未完成的镜像已清理。")
				app.appendLog("任务已取消；未完成的镜像已清理。")
			} else if err != nil {
				setText(app.statusLabel, "提取失败："+err.Error())
				app.appendLog("提取失败：" + err.Error())
				showMessage(app.hwnd, "提取失败", err.Error(), mbOK|mbIconError)
			} else {
				sendMessage(app.progressBar, pbmSetPos, 1000, 0)
				setText(app.statusLabel, "提取完成。镜像已保存到："+outputDir)
				app.appendLog("全部完成，保存目录：" + outputDir)
				showMessage(app.hwnd, "提取完成", "所选分区已成功提取。", mbOK|mbIconInfo)
			}
			if app.closeAfterCancel {
				procDestroyWindow.Call(app.hwnd)
			}
		})
	}()
}

func (app *application) showProgress(progress ExtractionProgress) {
	position := 0
	if progress.OverallSize > 0 {
		position = int(progress.OverallBytes * 1000 / progress.OverallSize)
	} else if progress.OverallTotal > 0 {
		position = progress.OverallDone * 1000 / progress.OverallTotal
	}
	if position < 0 {
		position = 0
	}
	if position > 1000 {
		position = 1000
	}
	sendMessage(app.progressBar, pbmSetPos, uintptr(position), 0)
	stage := progress.Stage
	if stage == "" {
		stage = "提取"
	}
	partitionProgress := fmt.Sprintf("%d/%d 项", progress.CompletedOps, progress.PartitionOps)
	if progress.BytesTotal > 0 {
		partitionProgress = fmt.Sprintf("%s / %s", formatBytes(progress.BytesDone), formatBytes(progress.BytesTotal))
	}
	setText(app.statusLabel, fmt.Sprintf("%s %s：%s（总进度 %.1f%%）", stage, progress.Partition, partitionProgress, float64(position)/10))
	if progress.PartitionDone {
		if progress.PartitionError != nil {
			app.appendLog(progress.Partition + ".img - 失败：" + progress.PartitionError.Error())
		} else {
			app.appendLog(progress.Partition + ".img - OK")
		}
	}
}

func (app *application) cancelExtraction() {
	if app.cancel == nil {
		return
	}
	app.cancel()
	enable(app.cancelButton, false)
	setText(app.statusLabel, "正在取消并清理临时文件...")
	app.appendLog("收到取消请求，正在安全停止并清理...")
}

func (app *application) setWorking(working, cancelable bool) {
	app.busy = working
	for _, hwnd := range []uintptr{app.inputEdit, app.loadButton, app.browseButton, app.searchEdit, app.partitionList, app.selectAllButton, app.selectNoneButton, app.outputEdit, app.outputButton, app.threadEdit, app.extractButton} {
		enable(hwnd, !working)
	}
	enable(app.cancelButton, working && cancelable)
}

func (app *application) appendLog(line string) {
	if line == "" {
		return
	}
	appendEditText(app.logEdit, "["+time.Now().Format("15:04:05")+"] "+line+"\r\n")
}

func formatBytes(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	index := -1
	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}
	if value >= 100 {
		return fmt.Sprintf("%.0f %s", value, units[index])
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}
