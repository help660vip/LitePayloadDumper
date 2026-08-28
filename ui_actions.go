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
				showMessage(app.hwnd, "请输入固件", "请粘贴在线 OTA URL，或选择本地固件文件。", mbOK|mbIconInfo)
			}
		}
	case idSelectAll:
		app.setAllChecks(true)
	case idSelectNone:
		app.setAllChecks(false)
	case idSelectBoot:
		app.selectBootPartitions()
	case idOutputBrowse:
		if folder := chooseFolder(app.hwnd, "选择分区镜像保存目录"); folder != "" {
			setText(app.outputEdit, folder)
		}
	case idSourceBrowse:
		if folder := chooseFolder(app.hwnd, "选择增量 OTA 对应的旧镜像目录"); folder != "" {
			setText(app.sourceEdit, folder)
		}
	case idExtract:
		app.startExtraction()
	case idCancel:
		app.cancelExtraction()
	}
}

func (app *application) loadFile(filename string) {
	filename = strings.TrimSpace(filename)
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
	app.details = nil
	app.partitions = nil
	app.visiblePartitions = nil
	app.partitionChecks = make(map[string]bool)
	setText(app.searchEdit, "")
	sendMessage(app.partitionList, lvmDeleteAllItems, 0, 0)
	app.setInfo(DeviceInfo{})
	setText(app.logEdit, "")
	setText(app.statusLabel, "正在读取 Payload 清单与固件元数据...")
	sendMessage(app.progressBar, pbmSetPos, 0, 0)
	app.appendLog("开始分析：" + filename)
	app.setWorking(true, false)

	go func() {
		details, err := inspectPackage(filename)
		app.queueUI(func() {
			if serial != app.loadSerial {
				return
			}
			app.setWorking(false, false)
			if err != nil {
				setText(app.statusLabel, "读取失败。")
				app.appendLog("读取失败：" + err.Error())
				showMessage(app.hwnd, "读取失败", err.Error(), mbOK|mbIconError)
				return
			}
			app.details = details
			app.partitions = details.Partitions
			app.populatePartitions()
			app.setInfo(details.Info)
			if details.Remote {
				app.appendLog("在线按需模式：只读取 ZIP 目录、Payload 清单和所选分区对应的字节段。")
			}
			setText(app.statusLabel, fmt.Sprintf("读取完成：共 %d 个分区，固件大小 %s。", len(details.Partitions), formatBytes(uint64(details.FileSize))))
			app.appendLog(fmt.Sprintf("读取完成：Payload v%d，%d 个分区。", details.Info.PayloadVersion, len(details.Partitions)))
			if details.Info.Fingerprint != "" {
				app.appendLog("构建指纹：" + details.Info.Fingerprint)
			}
			if details.Info.IsDelta {
				app.appendLog("提示：这是增量 OTA；标记为“需旧镜像”的分区必须提供旧镜像目录。")
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
			base = strings.TrimSuffix(name, urlpath.Ext(name))
		}
		if home, err := os.UserHomeDir(); err == nil {
			downloads := filepath.Join(home, "Downloads")
			if stat, statErr := os.Stat(downloads); statErr == nil && stat.IsDir() {
				parent = downloads
			}
		}
		if parent == "" {
			parent, _ = os.Getwd()
		}
	} else {
		base = strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		parent = filepath.Dir(input)
	}
	if strings.TrimSpace(base) == "" || base == "." || base == "/" {
		base = "payload"
	}
	return filepath.Join(parent, base+"_提取")
}

func (app *application) populatePartitions() {
	sendMessage(app.partitionList, lvmDeleteAllItems, 0, 0)
	app.visiblePartitions = filterPartitions(app.partitions, getText(app.searchEdit))
	for row, part := range app.visiblePartitions {
		status := "可提取"
		if len(part.UnsupportedOps) > 0 {
			status = "暂不支持：" + strings.Join(part.UnsupportedOps, ", ")
		} else if part.NeedsSource {
			status = "需旧镜像（增量）"
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

func (app *application) selectBootPartitions() {
	if app.busy {
		return
	}
	common := map[string]bool{"boot": true, "init_boot": true, "vendor_boot": true, "dtbo": true, "vbmeta": true, "vbmeta_system": true}
	app.partitionChecks = make(map[string]bool, len(app.partitions))
	for _, part := range app.partitions {
		app.partitionChecks[part.Name] = common[part.Name]
	}
	for row, part := range app.visiblePartitions {
		setListChecked(app.partitionList, row, common[part.Name])
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
		showMessage(app.hwnd, "尚未读取固件", "请先选择 OTA ZIP 或 payload.bin。", mbOK|mbIconInfo)
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
	sourceDir := strings.TrimSpace(getText(app.sourceEdit))
	for _, item := range items {
		if item.NeedsSource && sourceDir == "" {
			showMessage(app.hwnd, "需要旧镜像", "所选增量分区需要旧版本镜像。请选择旧镜像目录，目录内文件名应为 分区名.img。", mbOK|mbIconInfo)
			return
		}
	}
	if sourceDir != "" {
		if stat, err := os.Stat(sourceDir); err != nil || !stat.IsDir() {
			showMessage(app.hwnd, "旧镜像目录无效", "请选择存在的旧镜像目录。", mbOK|mbIconError)
			return
		}
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
	if app.details.Remote {
		setText(app.statusLabel, "在线按需连接中：正在读取 ZIP 目录与 Payload 清单，可随时取消...")
	} else {
		setText(app.statusLabel, fmt.Sprintf("正在提取 %d 个分区（%d 线程）...", len(names), threads))
	}
	app.appendLog(fmt.Sprintf("开始提取：%s；线程数：%d。", strings.Join(names, ", "), threads))

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
		err := extractPackage(ctx, app.details.Path, outputDir, sourceDir, names, threads, progressCallback)
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
	setText(app.statusLabel, "正在取消并清理未完成文件...")
	app.appendLog("收到取消请求，正在安全停止...")
}

func (app *application) setWorking(working, cancelable bool) {
	app.busy = working
	for _, hwnd := range []uintptr{app.inputEdit, app.loadButton, app.browseButton, app.searchEdit, app.partitionList, app.selectBootButton, app.selectAllButton, app.selectNoneButton, app.outputEdit, app.outputButton, app.sourceEdit, app.sourceButton, app.threadEdit, app.extractButton} {
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
