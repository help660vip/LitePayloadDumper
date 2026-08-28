# ✨ LitePayloadDumper - 在线 Payload 分区提取器 ✨

[![Release](https://img.shields.io/github/v/release/help660vip/LitePayloadDumper?include_prereleases&label=release)](https://github.com/help660vip/LitePayloadDumper/releases)
[![Downloads](https://img.shields.io/github/downloads/help660vip/LitePayloadDumper/total?label=downloads)](https://github.com/help660vip/LitePayloadDumper/releases)
[![Windows](https://img.shields.io/badge/Windows-7%20%7C%2010%20%7C%2011-0078D6?logo=windows)](https://github.com/help660vip/LitePayloadDumper/releases)
[![License](https://img.shields.io/github/license/help660vip/LitePayloadDumper)](LICENSE)

一个轻量、单文件、无运行库依赖的 Android OTA Payload 图形化提取工具。可直接粘贴在线 OTA URL，只下载所选分区所需的数据，不会先拉取整个固件。

> [!IMPORTANT]
>
> 请仅处理你有权访问和使用的固件。因不当使用产生的后果由使用者自行承担。
>
> 在线服务器不支持 HTTP Range 时，程序会停止并提示改用本地文件，绝不会偷偷退化成整包下载。

## 📖 支持情况

| 功能 | 支持情况 | 说明 |
| :--- | :---: | :--- |
| 本地 OTA ZIP | ✅ | 直接读取 `payload.bin` 和固件元数据 |
| 本地 `payload.bin` | ✅ | 读取分区清单并提取镜像 |
| 在线 OTA URL | ✅ | HTTP Range 按需读取 |
| 完整 OTA | ✅ | 直接提取所选分区 |
| 增量 OTA | ✅ | 需要提供对应版本的旧分区镜像 |
| 机型/代号/安卓版本 | ✅ | 同时解析 OTA metadata 与 Payload manifest |
| 系统版本/补丁日期 | ✅ | 无信息时显示 `—`，不猜测内容 |
| 默认分区选择 | 全不选 | 避免误提取大分区 |

## 💿 下载

从 [GitHub Releases](https://github.com/help660vip/LitePayloadDumper/releases) 下载对应版本：

| 文件 | 适用系统 | 构建方式 |
| :--- | :--- | :--- |
| `LitePayloadDumper_Win7.exe` | Windows 7 SP1 | Go 1.20.14，兼顾老系统 |
| `LitePayloadDumper_Win10-11.exe` | Windows 10 / 11 | 当前 Go 工具链 |

两个版本都是 64 位单文件 EXE，不需要安装 .NET、Java、Python、VC++ 运行库或其他框架，也不需要把 DLL 放在程序旁边。

## 🎈 特性

- 读取本地 OTA ZIP、`payload.bin` 或在线 HTTP/HTTPS OTA URL
- 显示机型、设备代号、安卓版本、系统版本、安全补丁日期、SDK 和构建日期
- 列出 Payload 内的分区、镜像大小、操作数和增量状态
- 支持按分区名称实时搜索，搜索时保留已有勾选状态
- 默认不勾选任何分区，支持“常用启动分区”“全选”“全不选”
- 只提取勾选的分区，线程数可输入 1～64，默认 4 线程
- 支持 REPLACE、XZ、Zstd、Bzip2、Brotli、BSDIFF 等常见 Payload 操作
- 取消任务时清理未完成的临时镜像
- 原生 Win32 界面，纯 Go 静态构建，无外置运行时

## 🎉 使用

| 步骤 | 操作 | 说明 |
| :---: | :--- | :--- |
| 1 | 粘贴 URL 或选择本地文件 | 支持拖入 OTA ZIP / `payload.bin` |
| 2 | 点击“读取” | 显示固件信息和分区清单 |
| 3 | 勾选需要的分区 | 初始状态全部不勾选 |
| 4 | 选择目录与线程数 | 可输入 1～64，默认 4；低配建议 1～2 |
| 5 | 点击“提取所选分区” | 输出为 `分区名.img` |

增量 OTA 中标记为“需旧镜像”的分区，需要在“旧镜像目录”中放置同名文件，例如 `system.img`。

## ⚙️ 在线按需模式

在线模式的工作流程：

1. 通过 Range 读取 ZIP 尾部目录，定位 `payload.bin`
2. 读取 Payload 文件头和分区清单
3. 提取时只请求所选分区操作对应的远程字节块
4. 使用有上限的 4 MiB 分块缓存，避免重复请求

> [!NOTE]
>
> 在线地址必须正确返回 `206 Partial Content`，并且 OTA ZIP 内的 `payload.bin` 必须是 Store（未二次压缩）。条件不满足时程序会提示改用本地文件。用于显示机型和版本的小型 metadata 文件也会按需读取。

## 🛠️ 从源码构建

需要 Windows PowerShell。Win7 版必须使用 Go 1.20.14；Win10/11 版可使用当前稳定版 Go：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 `
  -Go120 'C:\Go120\bin\go.exe' `
  -GoModern 'C:\Go\bin\go.exe'
```

产物位于 `dist`。构建强制 `CGO_ENABLED=0`，并使用纯 Go 的 XZ、Zstd、Bzip2 和 Brotli 实现，因此没有随附 DLL。

运行测试：

```powershell
go test ./...
```

如需验证真实在线 OTA，可设置 `ONLINE_OTA_URL` 后单独运行 `TestOnlineOTAFromEnvironment`。

## 🎉 致谢

- [ssut/payload-dumper-go](https://github.com/ssut/payload-dumper-go) — Payload 解析与提取核心
- [Android Open Source Project](https://source.android.com/) — Payload update metadata 定义
- 致每一位提交 Issue、测试固件和改进建议的用户

## 📄 许可

本项目自身代码采用 [MIT License](LICENSE)。Payload 核心基于 Apache-2.0 许可的 `payload-dumper-go`，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) 和 `third_party/payload-dumper-go/LICENSE`。
