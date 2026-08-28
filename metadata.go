package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DeviceInfo struct {
	Brand          string
	Model          string
	Device         string
	Android        string
	SDK            string
	SystemVersion  string
	SecurityPatch  string
	BuildDate      string
	Fingerprint    string
	PackageType    string
	PayloadVersion uint64
	MinorVersion   uint32
	IsDelta        bool
	PartitionCount int
}

type metadataValue struct {
	value string
	score int
}

var xmlPropertyPattern = regexp.MustCompile("(?is)name\\s*=\\s*['\"]([^'\"]+)['\"][^>]{0,500}?value\\s*=\\s*['\"]([^'\"]*)['\"]")

func inspectArchiveMetadata(filename string) DeviceInfo {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return DeviceInfo{PackageType: "payload.bin"}
	}
	defer zr.Close()
	return inspectZipMetadata(&zr.Reader)
}

func inspectZipMetadata(zr *zip.Reader) DeviceInfo {
	info := DeviceInfo{PackageType: "payload.bin"}
	if zr == nil {
		return info
	}
	info.PackageType = "OTA ZIP"

	values := make(map[string]metadataValue)
	put := func(key, value string, score int) {
		key = normalizeMetadataKey(key)
		value = cleanMetadataValue(value)
		if key == "" || value == "" || len(value) > 512 {
			return
		}
		old, exists := values[key]
		if !exists || score > old.score {
			values[key] = metadataValue{value: value, score: score}
		}
	}

	remaining := int64(20 << 20)
	for _, f := range zr.File {
		if remaining <= 0 || f.FileInfo().IsDir() || f.UncompressedSize64 == 0 || f.UncompressedSize64 > 4<<20 {
			continue
		}
		name := strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))
		if !isMetadataCandidate(name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		limit := int64(f.UncompressedSize64)
		if limit > remaining {
			limit = remaining
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, limit))
		rc.Close()
		if readErr != nil {
			continue
		}
		remaining -= int64(len(data))
		parseMetadataDocument(decodeMetadataText(data), metadataFileScore(name), put)
	}

	choose := func(keys ...string) string {
		best := metadataValue{}
		found := false
		for i, key := range keys {
			if candidate, ok := values[normalizeMetadataKey(key)]; ok {
				candidate.score += (len(keys) - i) * 1000
				if !found || candidate.score > best.score {
					best, found = candidate, true
				}
			}
		}
		if found {
			return best.value
		}
		return ""
	}

	info.Fingerprint = choose("post-build", "ro.build.fingerprint", "ro.system.build.fingerprint", "build_fingerprint")
	applyFingerprint(&info, info.Fingerprint)
	info.Brand = firstNonEmpty(choose("ro.product.brand", "ro.product.system.brand", "product_brand", "brand"), info.Brand)
	info.Model = firstNonEmpty(choose("ro.product.marketname", "ro.product.product.marketname", "ro.vendor.oplus.market.name", "ro.product.model", "ro.product.system.model", "product_model", "model_name", "model"), info.Model)
	info.Device = firstToken(firstNonEmpty(choose("ro.product.device", "ro.product.system.device", "ro.build.product", "pre-device", "product_device", "device", "product_name"), info.Device))
	info.Android = firstNonEmpty(choose("ro.build.version.release", "ro.system.build.version.release", "android_version", "android.version", "version.release"), info.Android)
	info.SDK = choose("post-sdk-level", "ro.build.version.sdk", "ro.system.build.version.sdk", "sdk_level", "sdk")
	info.SystemVersion = firstNonEmpty(choose("ota_version", "ota.version", "real_version", "real.version", "rom_version", "version_name", "ro.build.display.id", "ro.system.build.display.id", "ro.build.version.incremental", "post-build-incremental"), info.SystemVersion)
	info.SecurityPatch = choose("post-security-patch-level", "ro.build.version.security_patch", "ro.vendor.build.security_patch", "security_patch_level", "security_patch", "google_patch")
	if ts := choose("post-timestamp", "ro.build.date.utc", "build_timestamp", "timestamp"); ts != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64); err == nil {
			info.BuildDate = formatUnixDate(n)
		}
	}
	return info
}

func isMetadataCandidate(name string) bool {
	base := path.Base(name)
	if base == "payload.bin" || strings.HasSuffix(base, ".img") || strings.HasSuffix(base, ".dat") {
		return false
	}
	if name == "meta-inf/com/android/metadata" || base == "metadata" || base == "oplus_metadata" || base == "build.prop" || base == "system-build.prop" || base == "vendor-build.prop" || base == "payload_properties.txt" {
		return true
	}
	if strings.HasSuffix(base, ".prop") || strings.HasSuffix(base, ".properties") {
		return true
	}
	textLike := strings.HasSuffix(base, ".txt") || strings.HasSuffix(base, ".json") || strings.HasSuffix(base, ".xml") || strings.HasSuffix(base, ".conf") || strings.HasSuffix(base, ".cfg")
	return textLike && (strings.Contains(name, "metadata") || strings.Contains(name, "version") || strings.Contains(name, "build") || strings.Contains(name, "device") || strings.Contains(name, "ota"))
}

func metadataFileScore(name string) int {
	switch {
	case name == "meta-inf/com/android/metadata":
		return 500
	case path.Base(name) == "oplus_metadata":
		return 480
	case path.Base(name) == "build.prop":
		return 450
	case strings.HasSuffix(name, "-build.prop"):
		return 420
	case strings.HasSuffix(name, ".prop"):
		return 350
	case strings.HasSuffix(name, ".json"):
		return 260
	default:
		return 200
	}
}

func parseMetadataDocument(text string, score int, put func(string, string, int)) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if index := strings.IndexByte(line, '='); index > 0 {
			put(line[:index], line[index+1:], score)
			continue
		}
		if index := strings.IndexByte(line, ':'); index > 0 {
			key := strings.TrimSpace(line[:index])
			if !strings.ContainsAny(key, " /\\") {
				put(key, line[index+1:], score-10)
			}
		}
	}

	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		var root interface{}
		if json.Unmarshal([]byte(text), &root) == nil {
			flattenJSON("", root, func(key, value string) { put(key, value, score+20) })
		}
	}
	for _, match := range xmlPropertyPattern.FindAllStringSubmatch(text, -1) {
		if len(match) == 3 {
			put(html.UnescapeString(match[1]), html.UnescapeString(match[2]), score)
		}
	}
}

func flattenJSON(prefix string, value interface{}, put func(string, string)) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			full := key
			if prefix != "" {
				full = prefix + "." + key
			}
			flattenJSON(full, child, put)
		}
	case []interface{}:
		if len(typed) == 1 {
			flattenJSON(prefix, typed[0], put)
		}
	case string:
		put(prefix, typed)
	case float64, bool:
		put(prefix, fmt.Sprint(typed))
	}
}

func decodeMetadataText(data []byte) string {
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		return decodeUTF16(data[2:], binary.LittleEndian)
	}
	if len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff {
		return decodeUTF16(data[2:], binary.BigEndian)
	}
	if len(data) > 0 && bytes.Count(data, []byte{0}) > len(data)/4 {
		return decodeUTF16(data, binary.LittleEndian)
	}
	return string(data)
}

func decodeUTF16(data []byte, order binary.ByteOrder) string {
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(order.Uint16(data[i : i+2]))
		if r != 0 {
			runes = append(runes, r)
		}
	}
	return string(runes)
}

func normalizeMetadataKey(key string) string {
	key = strings.TrimSpace(strings.Trim(key, "\"'"))
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return key
}

func cleanMetadataValue(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "\"' ,;"))
	value = strings.ReplaceAll(value, "\\/", "/")
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstToken(value string) string {
	if index := strings.IndexAny(value, ",; "); index >= 0 {
		return value[:index]
	}
	return value
}

func applyFingerprint(info *DeviceInfo, fingerprint string) {
	if fingerprint == "" {
		return
	}
	left, right, found := strings.Cut(fingerprint, ":")
	products := strings.Split(left, "/")
	if len(products) >= 3 {
		info.Brand = products[0]
		info.Model = products[1]
		info.Device = products[2]
	}
	if !found {
		return
	}
	build := strings.Split(right, "/")
	if len(build) > 0 {
		info.Android = build[0]
	}
	if len(build) > 2 {
		info.SystemVersion = build[1] + "/" + build[2]
	} else if len(build) > 1 {
		info.SystemVersion = build[1]
	}
}

func formatUnixDate(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).UTC().Format("2006-01-02 15:04 UTC")
}
