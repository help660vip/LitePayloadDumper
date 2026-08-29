package mobileapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/help660vip/LitePayloadDumper/core"
)

const appVersion = "1.1.0"

type Listener interface {
	OnEvent(event string)
}

type Session struct {
	mu          sync.Mutex
	input       string
	displayName string
	details     *core.PackageDetails
	cancel      context.CancelFunc
	busy        bool
	closed      bool
}

type partitionJSON struct {
	Name        string `json:"name"`
	Size        uint64 `json:"size"`
	Operations  int    `json:"operations"`
	NeedsSource bool   `json:"needsSource"`
	Unsupported string `json:"unsupported,omitempty"`
}

type deviceInfoJSON struct {
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	Device         string `json:"device"`
	Android        string `json:"android"`
	SDK            string `json:"sdk"`
	SystemVersion  string `json:"systemVersion"`
	SecurityPatch  string `json:"securityPatch"`
	BuildDate      string `json:"buildDate"`
	Fingerprint    string `json:"fingerprint"`
	PackageType    string `json:"packageType"`
	PayloadVersion uint64 `json:"payloadVersion"`
	MinorVersion   uint32 `json:"minorVersion"`
	IsDelta        bool   `json:"isDelta"`
}

type detailsJSON struct {
	Input          string          `json:"input"`
	FileSize       int64           `json:"fileSize"`
	Remote         bool            `json:"remote"`
	Mode           string          `json:"mode"`
	ArchiveEntries int             `json:"archiveEntries"`
	Partitions     []partitionJSON `json:"partitions"`
	Info           deviceInfoJSON  `json:"info"`
}

type eventJSON struct {
	Type          string  `json:"type"`
	Stage         string  `json:"stage,omitempty"`
	Partition     string  `json:"partition,omitempty"`
	Done          uint64  `json:"done,omitempty"`
	Total         uint64  `json:"total,omitempty"`
	OverallDone   int     `json:"overallDone,omitempty"`
	OverallTotal  int     `json:"overallTotal,omitempty"`
	BytesDone     uint64  `json:"bytesDone,omitempty"`
	BytesTotal    uint64  `json:"bytesTotal,omitempty"`
	OverallBytes  uint64  `json:"overallBytes,omitempty"`
	OverallSize   uint64  `json:"overallSize,omitempty"`
	Percent       float64 `json:"percent,omitempty"`
	PartitionDone bool    `json:"partitionDone,omitempty"`
	Error         string  `json:"error,omitempty"`
}

func AppVersion() string {
	return appVersion
}

func SetTempDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("临时目录不能为空")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("无法创建临时目录：%w", err)
	}
	return os.Setenv("TMPDIR", path)
}

func NewSession(input, displayName string) *Session {
	return &Session{input: strings.TrimSpace(input), displayName: strings.TrimSpace(displayName)}
}

func (session *Session) begin() (context.Context, func(), error) {
	if session == nil {
		return nil, nil, errors.New("会话不存在")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return nil, nil, errors.New("会话已关闭")
	}
	if session.busy {
		return nil, nil, errors.New("已有任务正在执行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	session.busy = true
	session.cancel = cancel
	finish := func() {
		cancel()
		session.mu.Lock()
		session.busy = false
		session.cancel = nil
		session.mu.Unlock()
	}
	return ctx, finish, nil
}

func (session *Session) Inspect(listener Listener) (string, error) {
	ctx, finish, err := session.begin()
	if err != nil {
		return "", err
	}
	defer finish()
	if session.input == "" {
		return "", errors.New("请选择固件文件或输入在线地址")
	}
	details, err := core.InspectPackageContext(ctx, session.input, func(progress core.InspectionProgress) {
		emit(listener, eventJSON{Type: "inspection", Stage: progress.Stage, Done: progress.Done, Total: progress.Total, Percent: percent(progress.Done, progress.Total)})
	})
	if err != nil {
		emit(listener, eventJSON{Type: "error", Error: err.Error()})
		return "", err
	}
	if session.displayName != "" {
		core.ApplyArchiveFilenameMetadata(&details.Info, session.displayName)
	}
	session.mu.Lock()
	oldDetails := session.details
	session.details = details
	session.mu.Unlock()
	if oldDetails != nil {
		oldDetails.Cleanup()
	}
	result, err := marshalDetails(details, session.displayName)
	if err != nil {
		return "", err
	}
	emit(listener, eventJSON{Type: "inspected", OverallTotal: len(details.Partitions)})
	return result, nil
}

func (session *Session) Extract(outputDir, partitionsJSON string, concurrency int, listener Listener) error {
	ctx, finish, err := session.begin()
	if err != nil {
		return err
	}
	defer finish()
	if concurrency < 1 || concurrency > 64 {
		return errors.New("线程数必须是 1～64 的整数")
	}
	var names []string
	if err := json.Unmarshal([]byte(partitionsJSON), &names); err != nil {
		return fmt.Errorf("分区列表无效：%w", err)
	}
	if len(names) == 0 {
		return errors.New("请至少选择一个分区")
	}
	session.mu.Lock()
	details := session.details
	session.mu.Unlock()
	if details == nil {
		return errors.New("请先读取固件")
	}
	err = core.ExtractPackageWithItems(ctx, details.Path, outputDir, "", names, details.Partitions, concurrency, func(progress core.ExtractionProgress) {
		failure := ""
		if progress.PartitionError != nil {
			failure = progress.PartitionError.Error()
		}
		emit(listener, eventJSON{
			Type:          "extraction",
			Stage:         progress.Stage,
			Partition:     progress.Partition,
			OverallDone:   progress.OverallDone,
			OverallTotal:  progress.OverallTotal,
			BytesDone:     progress.BytesDone,
			BytesTotal:    progress.BytesTotal,
			OverallBytes:  progress.OverallBytes,
			OverallSize:   progress.OverallSize,
			Percent:       percent(progress.OverallBytes, progress.OverallSize),
			PartitionDone: progress.PartitionDone,
			Error:         failure,
		})
	})
	if err != nil {
		emit(listener, eventJSON{Type: "error", Error: err.Error()})
		return err
	}
	emit(listener, eventJSON{Type: "complete", Percent: 100})
	return nil
}

func (session *Session) Cancel() {
	if session == nil {
		return
	}
	session.mu.Lock()
	cancel := session.cancel
	session.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (session *Session) Close() {
	if session == nil {
		return
	}
	session.Cancel()
	session.mu.Lock()
	session.closed = true
	details := session.details
	session.details = nil
	session.mu.Unlock()
	if details != nil {
		details.Cleanup()
	}
}

func marshalDetails(details *core.PackageDetails, displayName string) (string, error) {
	partitions := make([]partitionJSON, 0, len(details.Partitions))
	for _, item := range details.Partitions {
		partitions = append(partitions, partitionJSON{
			Name: item.Name, Size: item.Size, Operations: item.Operations, NeedsSource: item.NeedsSource,
			Unsupported: strings.Join(item.UnsupportedOps, ", "),
		})
	}
	info := details.Info
	input := details.OriginalPath
	if displayName != "" {
		input = displayName
	}
	encoded, err := json.Marshal(detailsJSON{
		Input: input, FileSize: details.FileSize, Remote: details.Remote, Mode: string(details.Mode),
		ArchiveEntries: details.ArchiveEntries, Partitions: partitions,
		Info: deviceInfoJSON{
			Brand: info.Brand, Model: info.Model, Device: info.Device, Android: info.Android, SDK: info.SDK,
			SystemVersion: info.SystemVersion, SecurityPatch: info.SecurityPatch, BuildDate: info.BuildDate,
			Fingerprint: info.Fingerprint, PackageType: info.PackageType, PayloadVersion: info.PayloadVersion,
			MinorVersion: info.MinorVersion, IsDelta: info.IsDelta,
		},
	})
	if err != nil {
		return "", fmt.Errorf("生成固件信息失败：%w", err)
	}
	return string(encoded), nil
}

func emit(listener Listener, event eventJSON) {
	if listener == nil {
		return
	}
	encoded, err := json.Marshal(event)
	if err == nil {
		listener.OnEvent(string(encoded))
	}
}

func percent(done, total uint64) float64 {
	if total == 0 {
		return 0
	}
	value := float64(done) * 100 / float64(total)
	if value > 100 {
		return 100
	}
	return value
}
