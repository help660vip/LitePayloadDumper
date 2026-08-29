<p align="center">
  <img src="assets/logo-mark.png" width="112" alt="LitePayloadDumper Logo">
</p>

# LitePayloadDumper

[![Release](https://img.shields.io/github/v/release/help660vip/LitePayloadDumper?label=release)](https://github.com/help660vip/LitePayloadDumper/releases)
[![Windows](https://img.shields.io/badge/Windows-7%20%7C%2010%20%7C%2011-0078D6?logo=windows)](https://github.com/help660vip/LitePayloadDumper/releases)
[![License](https://img.shields.io/github/license/help660vip/LitePayloadDumper)](LICENSE)

LitePayloadDumper 是一个 Windows 图形化 Android 固件分区提取工具。它可以读取本地文件或在线链接，从 OTA Payload、线刷 ZIP、TGZ / TAR.GZ 中选择并导出分区镜像。

## 下载

前往 [Releases](https://github.com/help660vip/LitePayloadDumper/releases) 下载：

| 文件 | 系统 |
| --- | --- |
| `LitePayloadDumper_Win7.exe` | Windows 7 SP1 64 位 |
| `LitePayloadDumper_Win10-11.exe` | Windows 10 / 11 64 位 |

两个版本均为单文件 EXE，不需要安装 .NET、Java、Python、VC++ 运行库，也不需要外置 DLL。

## 使用方法

1. 选择本地固件，或粘贴 HTTP / HTTPS 链接。
2. 点击“读取”，等待分区列表出现。
3. 使用搜索框查找分区并勾选需要的项目。程序默认不勾选任何分区。
4. 设置保存目录和线程数。线程范围为 1～64，默认 4。
5. 点击“提取所选分区”。

界面会显示固件中的机型、设备代号、Android 版本、系统版本、安全补丁日期等可识别信息。需要旧版本镜像的增量分区不支持提取，请使用完整固件。

## 支持的文件

- 含 `payload.bin` 的 OTA ZIP
- 独立 `payload.bin`
- 直接包含 `.img` 的线刷 ZIP
- TGZ / TAR.GZ 线刷包

在线 ZIP 使用 HTTP Range 按需读取。服务器必须返回 `206 Partial Content`；不支持 Range 时程序会停止，不会改为下载整个 ZIP。远程 ZIP 没有 `payload.bin` 时，会自动按普通 ZIP 读取其中的 `.img`。

TGZ 是连续 gzip 数据，无法随机定位单个文件。读取远程 TGZ 前程序会先说明情况并征求确认；确认后完整缓存一次，提取时复用缓存，取消、换包或退出时清理临时文件。

## 从源码构建

Windows 7 版本使用 Go 1.20.14；Windows 10 / 11 版本使用当前稳定版 Go：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 `
  -Go120 'C:\Go120\bin\go.exe' `
  -GoModern 'C:\Go\bin\go.exe'
```

运行测试：

```powershell
go test ./...
```

## 致谢与许可

Payload 解析核心基于 [ssut/payload-dumper-go](https://github.com/ssut/payload-dumper-go)。项目代码采用 [MIT License](LICENSE)，第三方许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
