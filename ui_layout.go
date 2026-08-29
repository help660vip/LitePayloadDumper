package main

const infoLabelWidth int32 = 104

func (app *application) createControls() {
	child := uint32(wsChild | wsVisible)
	button := child | wsTabStop | bsPushButton
	app.titleLabel = createWindow(0, "STATIC", "LitePayloadDumper", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.subtitleLabel = createWindow(0, "STATIC", "Android 固件分区镜像提取", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.githubLink = createWindow(0, "STATIC", projectLabel, child|ssNotify|ssOwnerDraw, 0, 0, 0, 0, app.hwnd, idGitHub, app.instance)
	app.inputLabel = createWindow(0, "STATIC", "固件文件或在线地址", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.inputEdit = createWindow(wsExClientEdge, "EDIT", "", child|wsTabStop|esAutoHScroll, 0, 0, 0, 0, app.hwnd, idInput, app.instance)
	app.loadButton = createWindow(0, "BUTTON", "读取", button, 0, 0, 0, 0, app.hwnd, idLoad, app.instance)
	app.browseButton = createWindow(0, "BUTTON", "打开文件...", button, 0, 0, 0, 0, app.hwnd, idBrowse, app.instance)

	app.infoGroup = createWindow(0, "BUTTON", "固件信息", child|0x00000007, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	labels := []string{"机型", "系统版本", "设备代号", "安卓版本", "补丁日期", "SDK", "包类型", "构建日期"}
	for i, label := range labels {
		app.infoLabels[i] = createWindow(0, "STATIC", label+"：", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
		app.infoValues[i] = createWindow(0, "STATIC", "—", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	}

	app.partitionLabel = createWindow(0, "STATIC", "分区列表", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.searchLabel = createWindow(0, "STATIC", "搜索分区：", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.searchEdit = createWindow(wsExClientEdge, "EDIT", "", child|wsTabStop|esAutoHScroll, 0, 0, 0, 0, app.hwnd, idSearch, app.instance)
	sendMessage(app.searchEdit, emSetLimitText, 128, 0)
	app.selectAllButton = createWindow(0, "BUTTON", "全选", button, 0, 0, 0, 0, app.hwnd, idSelectAll, app.instance)
	app.selectNoneButton = createWindow(0, "BUTTON", "全不选", button, 0, 0, 0, 0, app.hwnd, idSelectNone, app.instance)
	app.partitionList = createWindow(wsExClientEdge, "SysListView32", "", child|wsTabStop|wsVScroll|wsHScroll|lvsReport|lvsShowSelAlways, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	listStyles := uintptr(lvsExFullRowSelect | lvsExCheckBoxes | lvsExDoubleBuffer)
	sendMessage(app.partitionList, lvmSetExtendedStyle, listStyles, listStyles)
	addListColumn(app.partitionList, 0, "分区", 285, false)
	addListColumn(app.partitionList, 1, "镜像大小", 125, true)
	addListColumn(app.partitionList, 2, "操作数", 75, true)
	addListColumn(app.partitionList, 3, "状态", 390, false)

	app.outputLabel = createWindow(0, "STATIC", "保存目录：", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.outputEdit = createWindow(wsExClientEdge, "EDIT", "", child|wsTabStop|esAutoHScroll, 0, 0, 0, 0, app.hwnd, idOutput, app.instance)
	app.outputButton = createWindow(0, "BUTTON", "选择...", button, 0, 0, 0, 0, app.hwnd, idOutputBrowse, app.instance)
	app.threadLabel = createWindow(0, "STATIC", "线程：", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.threadEdit = createWindow(wsExClientEdge, "EDIT", "4", child|wsTabStop|esAutoHScroll|esNumber, 0, 0, 0, 0, app.hwnd, idThreads, app.instance)
	sendMessage(app.threadEdit, emSetLimitText, 2, 0)
	app.extractButton = createWindow(0, "BUTTON", "提取所选分区", button|bsDefPushButton, 0, 0, 0, 0, app.hwnd, idExtract, app.instance)
	app.cancelButton = createWindow(0, "BUTTON", "取消", button, 0, 0, 0, 0, app.hwnd, idCancel, app.instance)
	enable(app.cancelButton, false)
	app.statusLabel = createWindow(0, "STATIC", "选择固件文件，或粘贴在线地址。", child|ssLeft, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	app.progressBar = createWindow(0, "msctls_progress32", "", child, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	sendMessage(app.progressBar, pbmSetRange32, 0, 1000)
	app.logEdit = createWindow(wsExClientEdge, "EDIT", "", child|wsVScroll|esMultiline|esReadOnly|esAutoVScroll, 0, 0, 0, 0, app.hwnd, 0, app.instance)
	sendMessage(app.logEdit, emSetLimitText, 1024*1024, 0)

	all := []uintptr{app.titleLabel, app.subtitleLabel, app.githubLink, app.inputLabel, app.inputEdit, app.loadButton, app.browseButton, app.infoGroup, app.partitionLabel, app.searchLabel, app.searchEdit, app.selectAllButton, app.selectNoneButton, app.partitionList, app.outputLabel, app.outputEdit, app.outputButton, app.threadLabel, app.threadEdit, app.extractButton, app.cancelButton, app.statusLabel, app.progressBar, app.logEdit}
	all = append(all, app.infoLabels[:]...)
	all = append(all, app.infoValues[:]...)
	for _, control := range all {
		applyFont(control, app.font)
	}
}

func (app *application) layout() {
	if app == nil || app.hwnd == 0 || app.titleLabel == 0 {
		return
	}
	width, height := getClientSize(app.hwnd)
	if width <= 0 || height <= 0 {
		return
	}
	margin := int32(18)
	move(app.titleLabel, margin, 12, width-2*margin, 27)
	move(app.subtitleLabel, margin, 41, width-2*margin, 22)
	move(app.inputLabel, margin, 67, width-2*margin, 21)
	move(app.inputEdit, margin, 90, width-2*margin-194, 28)
	move(app.loadButton, width-margin-186, 89, 86, 29)
	move(app.browseButton, width-margin-92, 89, 92, 29)
	move(app.infoGroup, margin, 126, width-2*margin, 118)

	leftX := margin + 16
	rightX := width/2 + 4
	leftValueWidth := width/2 - leftX - infoLabelWidth - 14
	rightValueWidth := width - margin - rightX - infoLabelWidth - 12
	for row := 0; row < 4; row++ {
		y := int32(148 + row*23)
		left := row * 2
		right := left + 1
		move(app.infoLabels[left], leftX, y, infoLabelWidth, 21)
		move(app.infoValues[left], leftX+infoLabelWidth, y, leftValueWidth, 21)
		move(app.infoLabels[right], rightX, y, infoLabelWidth, 21)
		move(app.infoValues[right], rightX+infoLabelWidth, y, rightValueWidth, 21)
	}

	move(app.partitionLabel, margin, 254, 100, 24)
	move(app.searchLabel, margin+104, 254, 94, 24)
	searchX := margin + 202
	searchRight := width - margin - 168
	searchWidth := searchRight - searchX
	if searchWidth < 100 {
		searchWidth = 100
	}
	move(app.searchEdit, searchX, 250, searchWidth, 28)
	move(app.selectAllButton, width-margin-160, 250, 74, 28)
	move(app.selectNoneButton, width-margin-80, 250, 80, 28)
	listBottom := height - 226
	move(app.partitionList, margin, 282, width-2*margin, listBottom-282)

	move(app.outputLabel, margin, height-216, 108, 25)
	move(app.outputEdit, margin+110, height-219, width-2*margin-210, 28)
	move(app.outputButton, width-margin-92, height-219, 92, 29)
	move(app.threadLabel, margin, height-180, 68, 25)
	move(app.threadEdit, margin+70, height-183, 72, 28)
	move(app.extractButton, width-margin-222, height-184, 134, 31)
	move(app.cancelButton, width-margin-82, height-184, 82, 31)
	move(app.statusLabel, margin, height-148, width-2*margin, 21)
	move(app.progressBar, margin, height-126, width-2*margin, 17)
	move(app.logEdit, margin, height-102, width-2*margin, 55)
	githubWidth := measureTextWidth(app.font, projectLabel) + 46
	if githubWidth < 340 {
		githubWidth = 340
	}
	if githubWidth > width-2*margin {
		githubWidth = width - 2*margin
	}
	move(app.githubLink, (width-githubWidth)/2, height-40, githubWidth, 25)
}
