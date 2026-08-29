package main

import (
	"path/filepath"
	"testing"
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
