package main

import (
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

func TestFilterPartitionsByName(t *testing.T) {
	partitions := []PartitionItem{
		{Name: "boot"},
		{Name: "init_boot"},
		{Name: "system"},
		{Name: "vendor_boot"},
	}
	tests := []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"boot", "init_boot", "system", "vendor_boot"}},
		{query: "boot", want: []string{"boot", "init_boot", "vendor_boot"}},
		{query: "INIT", want: []string{"init_boot"}},
		{query: " system ", want: []string{"system"}},
		{query: "missing", want: nil},
	}
	for _, test := range tests {
		got := filterPartitions(partitions, test.query)
		if len(got) != len(test.want) {
			t.Fatalf("query %q returned %d items, want %d", test.query, len(got), len(test.want))
		}
		for index, name := range test.want {
			if got[index].Name != name {
				t.Fatalf("query %q item %d = %q, want %q", test.query, index, got[index].Name, name)
			}
		}
	}
}

func TestParseThreadCount(t *testing.T) {
	for _, input := range []string{"1", "4", "64", " 8 "} {
		if _, err := parseThreadCount(input); err != nil {
			t.Fatalf("parseThreadCount(%q) returned %v", input, err)
		}
	}
	for _, input := range []string{"", "0", "65", "-1", "abc", "4.5"} {
		if _, err := parseThreadCount(input); err == nil {
			t.Fatalf("parseThreadCount(%q) unexpectedly succeeded", input)
		}
	}
}

func TestDefaultOutputDirRemovesTarGZSuffix(t *testing.T) {
	for _, input := range []string{
		filepath.Join(t.TempDir(), "fastboot.tar.gz"),
		"https://example.test/releases/fastboot.tar.gz",
	} {
		if got := filepath.Base(defaultOutputDir(input)); got != "fastboot_提取" {
			t.Fatalf("defaultOutputDir(%q) = %q", input, got)
		}
	}
}

func TestRemoteOutputParentPrefersRelocatedDownloads(t *testing.T) {
	if got := chooseRemoteOutputParent(`D:\Downloads`, `E:\Tools`, true); got != `D:\Downloads` {
		t.Fatalf("relocated Downloads returned %q", got)
	}
}

func TestRemoteOutputParentFallsBackToExecutableDirectory(t *testing.T) {
	tests := []struct {
		name            string
		downloads       string
		downloadsUsable bool
	}{
		{name: "Downloads is on C", downloads: `C:\Users\tester\Downloads`, downloadsUsable: true},
		{name: "Known Folder unavailable", downloads: "", downloadsUsable: false},
		{name: "Downloads cannot be read", downloads: `D:\Downloads`, downloadsUsable: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chooseRemoteOutputParent(test.downloads, `E:\Portable\LitePayloadDumper`, test.downloadsUsable); got != `E:\Portable\LitePayloadDumper` {
				t.Fatalf("fallback directory = %q", got)
			}
		})
	}
}

func TestInfoLabelsHaveRoomForFullChineseText(t *testing.T) {
	getDC := user32.NewProc("GetDC")
	releaseDC := user32.NewProc("ReleaseDC")
	selectObject := gdi32.NewProc("SelectObject")
	getTextExtent := gdi32.NewProc("GetTextExtentPoint32W")
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		t.Fatal("failed to create screen device context")
	}
	font := createUIFont()
	if font == 0 {
		releaseDC.Call(0, dc)
		t.Fatal("failed to create UI font")
	}
	previous, _, _ := selectObject.Call(dc, font)
	defer func() {
		if previous != 0 {
			selectObject.Call(dc, previous)
		}
		procDeleteObject.Call(font)
		releaseDC.Call(0, dc)
	}()

	for _, label := range []string{"机型：", "系统版本：", "设备代号：", "安卓版本：", "补丁日期：", "SDK：", "包类型：", "构建日期："} {
		text := utf16Buffer(label)
		var extent point
		ok, _, _ := getTextExtent.Call(dc, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1), uintptr(unsafe.Pointer(&extent)))
		if ok == 0 {
			t.Fatalf("failed to measure %q", label)
		}
		if extent.X+8 > infoLabelWidth {
			t.Fatalf("label %q needs %d px including padding, layout provides %d px", label, extent.X+8, infoLabelWidth)
		}
	}
}

func TestGitHubFooterMarkAndText(t *testing.T) {
	if len(githubMarkRows) != 16 {
		t.Fatalf("GitHub mark has %d rows, want 16", len(githubMarkRows))
	}
	for row, bits := range githubMarkRows {
		if bits == 0 {
			t.Fatalf("GitHub mark row %d is empty", row)
		}
	}
	if projectURL != "https://github.com/help660vip/LitePayloadDumper" {
		t.Fatalf("unexpected project URL: %s", projectURL)
	}
}

func TestGitHubFooterNativeLayoutAndStyle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, _ := procGetModuleHandle.Call(0)
	parent := createWindow(0, "STATIC", "", wsOverlappedWindow, 0, 0, 1000, 780, 0, 0, instance)
	if parent == 0 {
		t.Fatal("failed to create native parent window")
	}
	defer procDestroyWindow.Call(parent)

	app := &application{hwnd: parent, instance: instance, font: createUIFont()}
	if app.font == 0 {
		t.Fatal("failed to create UI font")
	}
	defer procDeleteObject.Call(app.font)
	app.createControls()
	app.layout()

	if text := getText(app.githubLink); text != projectLabel {
		t.Fatalf("GitHub footer text = %q", text)
	}
	getWindowLongPtr := user32.NewProc("GetWindowLongPtrW")
	style, _, _ := getWindowLongPtr.Call(app.githubLink, ^uintptr(15))
	if style&ssNotify == 0 || style&0x1f != ssOwnerDraw {
		t.Fatalf("GitHub footer style %#x is not an owner-drawn clickable static", style)
	}

	getWindowRect := user32.NewProc("GetWindowRect")
	screenToClient := user32.NewProc("ScreenToClient")
	var bounds rect
	if ok, _, _ := getWindowRect.Call(app.githubLink, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		t.Fatal("failed to read GitHub footer bounds")
	}
	topLeft := point{X: bounds.Left, Y: bounds.Top}
	bottomRight := point{X: bounds.Right, Y: bounds.Bottom}
	screenToClient.Call(parent, uintptr(unsafe.Pointer(&topLeft)))
	screenToClient.Call(parent, uintptr(unsafe.Pointer(&bottomRight)))
	clientWidth, clientHeight := getClientSize(parent)
	footerWidth := bottomRight.X - topLeft.X
	textWidth := measureTextWidth(app.font, projectLabel)
	if footerWidth < textWidth+46 {
		t.Fatalf("GitHub footer width %d cannot fit text %d plus icon and padding", footerWidth, textWidth)
	}
	if bottomRight.Y != clientHeight-15 {
		t.Fatalf("GitHub footer bottom = %d, client height = %d", bottomRight.Y, clientHeight)
	}
	if topLeft.X != (clientWidth-footerWidth)/2 {
		t.Fatalf("GitHub footer is not horizontally centered: left=%d client=%d", topLeft.X, clientWidth)
	}
}

func TestDrawGitHubLinkRendersMark(t *testing.T) {
	type bitmapInfoHeader struct {
		Size          uint32
		Width         int32
		Height        int32
		Planes        uint16
		BitCount      uint16
		Compression   uint32
		SizeImage     uint32
		XPelsPerMeter int32
		YPelsPerMeter int32
		ClrUsed       uint32
		ClrImportant  uint32
	}
	type bitmapInfo struct {
		Header bitmapInfoHeader
		Colors [1]uint32
	}

	getDC := user32.NewProc("GetDC")
	releaseDC := user32.NewProc("ReleaseDC")
	createCompatibleDC := gdi32.NewProc("CreateCompatibleDC")
	createDIBSection := gdi32.NewProc("CreateDIBSection")
	deleteDC := gdi32.NewProc("DeleteDC")
	gdiFlush := gdi32.NewProc("GdiFlush")

	screenDC, _, _ := getDC.Call(0)
	if screenDC == 0 {
		t.Fatal("failed to get screen DC")
	}
	defer releaseDC.Call(0, screenDC)
	memoryDC, _, _ := createCompatibleDC.Call(screenDC)
	if memoryDC == 0 {
		t.Fatal("failed to create memory DC")
	}
	defer deleteDC.Call(memoryDC)
	info := bitmapInfo{}
	info.Header.Size = uint32(unsafe.Sizeof(info.Header))
	info.Header.Width = 278
	info.Header.Height = -25
	info.Header.Planes = 1
	info.Header.BitCount = 32
	var pixels uintptr
	bitmap, _, _ := createDIBSection.Call(
		screenDC,
		uintptr(unsafe.Pointer(&info)),
		0,
		uintptr(unsafe.Pointer(&pixels)),
		0,
		0,
	)
	if bitmap == 0 {
		t.Fatal("failed to create footer DIB section")
	}
	if pixels == 0 {
		t.Fatal("footer DIB section returned no pixel buffer")
	}
	previousBitmap, _, _ := procSelectObject.Call(memoryDC, bitmap)
	defer func() {
		procSelectObject.Call(memoryDC, previousBitmap)
		procDeleteObject.Call(bitmap)
	}()
	font := createUIFont()
	if font == 0 {
		t.Fatal("failed to create footer font")
	}
	defer procDeleteObject.Call(font)

	item := &drawItemStruct{DC: memoryDC, ItemRect: rect{Right: 278, Bottom: 25}}
	drawGitHubLink(item, font)
	gdiFlush.Call()
	buffer := unsafe.Slice((*uint32)(unsafe.Pointer(pixels)), 278*25)
	background := buffer[0]
	markPixels := 0
	for y := 4; y < 20; y++ {
		for x := 5; x < 21; x++ {
			if buffer[y*278+x] != background {
				markPixels++
			}
		}
	}
	if markPixels < 32 {
		t.Fatalf("GitHub mark contains only %d foreground pixels", markPixels)
	}
}

func TestEditableControlsForSelectAllShortcut(t *testing.T) {
	app := &application{inputEdit: 11, searchEdit: 12, outputEdit: 13, threadEdit: 14}
	for _, handle := range []uintptr{11, 12, 13, 14} {
		if !app.isEditableControl(handle) {
			t.Fatalf("editable handle %d was not recognized", handle)
		}
	}
	for _, handle := range []uintptr{0, 10, 15} {
		if app.isEditableControl(handle) {
			t.Fatalf("non-editable handle %d was recognized", handle)
		}
	}
	valid := &msg{Hwnd: 11, Message: wmKeyDown, WParam: vkA}
	if !app.isSelectAllShortcut(valid, true) {
		t.Fatal("Ctrl+A was not accepted for an editable control")
	}
	for _, candidate := range []struct {
		message     *msg
		controlDown bool
	}{
		{message: valid, controlDown: false},
		{message: &msg{Hwnd: 15, Message: wmKeyDown, WParam: vkA}, controlDown: true},
		{message: &msg{Hwnd: 11, Message: wmKeyDown, WParam: vkDelete}, controlDown: true},
		{message: nil, controlDown: true},
	} {
		if app.isSelectAllShortcut(candidate.message, candidate.controlDown) {
			t.Fatal("invalid Ctrl+A shortcut was accepted")
		}
	}
}

func TestNativeEditSelectAllThenDelete(t *testing.T) {
	instance, _, _ := procGetModuleHandle.Call(0)
	edit := createWindow(0, "EDIT", "需要清空的内容", esAutoHScroll, 0, 0, 300, 30, 0, 0, instance)
	if edit == 0 {
		t.Fatal("failed to create native EDIT control")
	}
	defer procDestroyWindow.Call(edit)
	sendMessage(edit, emSetSel, 0, ^uintptr(0))
	sendMessage(edit, wmKeyDown, vkDelete, 0)
	if value := getText(edit); value != "" {
		t.Fatalf("Ctrl+A selection followed by Delete left %q", value)
	}
}

func TestEmbeddedApplicationIconLoads(t *testing.T) {
	instance, _, _ := procGetModuleHandle.Call(0)
	for _, size := range []int32{16, 32} {
		icon := loadIconResource(instance, 1, size, size)
		if icon == 0 {
			t.Fatalf("embedded application icon %dx%d did not load", size, size)
		}
		procDestroyIcon.Call(icon)
	}
}
