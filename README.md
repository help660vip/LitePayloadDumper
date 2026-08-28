# ✨ LitePayloadDumper - Android 固件分区提取器 ✨

[![Release](https://img.shields.io/github/v/release/help660vip/LitePayloadDumper?include_prereleases&label=release)](https://github.com/help660vip/LitePayloadDumper/releases)
[![Downloads](https://img.shields.io/github/downloads/help660vip/LitePayloadDumper/total?label=downloads)](https://github.com/help660vip/LitePayloadDumper/releases)
[![Windows](https://img.shields.io/badge/Windows-7%20%7C%2010%20%7C%2011-0078D6?logo=windows)](https://github.com/help660vip/LitePayloadDumper/releases)
[![License](https://img.shields.io/github/license/help660vip/LitePayloadDumper)](LICENSE)

一个轻量、单文件、无运行库依赖的 Android 固件图形化提取工具。支持 OTA Payload、通用线刷 ZIP 和 TGZ，可直接粘贴在线 URL，并显示机型、代号、安卓版本、系统版本与补丁日期等信息。

> [!IMPORTANT]
>
> 请仅处理你有权访问和使用的固件。因不当使用产生的后果由使用者自行承担。
>
> 在线 ZIP 服务器不支持 HTTP Range 时，程序会停止并提示改用本地文件，绝不会偷偷退化成整包下载。TGZ 无法随机读取，程序会先明确询问，只有确认后才完整缓存一次。

## 📖 支持情况

| 功能 | 支持情况 | 说明 |
| :--- | :---: | :--- |
| 本地 OTA ZIP | ✅ | 直接读取 `payload.bin` 和固件元数据 |
| 本地 `payload.bin` | ✅ | 读取分区清单并提取镜像 |
| 在线 OTA URL | ✅ | HTTP Range 按需读取 Payload 数据 |
| 通用线刷 ZIP | ✅ | 无 `payload.bin` 时自动列出包内 `.img` |
| 在线通用 ZIP | ✅ | Range 读取目录，只下载所选镜像的压缩数据 |
| 本地 TGZ / TAR.GZ | ✅ | 顺序扫描并提取所选 `.img` |
| 在线 TGZ / TAR.GZ | ✅ | 确认后完整缓存一次，提取不重复下载 |
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

- 读取本地或在线 OTA ZIP、`payload.bin`、通用线刷 ZIP、TGZ / TAR.GZ
- 显示机型、设备代号、安卓版本、系统版本、安全补丁日期、SDK 和构建日期
- 列出 Payload 分区或线刷包内的 `.img` 镜像、大小与支持状态
- 支持按分区名称实时搜索，搜索时保留已有勾选状态
- 默认不勾选任何分区，支持“常用启动分区”“全选”“全不选”
- 只提取勾选的分区；Payload / ZIP 线程数可输入 1～64，默认 4 线程
- 支持 REPLACE、XZ、Zstd、Bzip2、Brotli、BSDIFF 等常见 Payload 操作
- 取消任务时清理未完成的临时镜像
- 原生 Win32 界面，纯 Go 静态构建，无外置运行时

## 🎉 使用

| 步骤 | 操作 | 说明 |
| :---: | :--- | :--- |
| 1 | 粘贴 URL 或选择本地文件 | 支持 OTA ZIP、`payload.bin`、线刷 ZIP、TGZ / TAR.GZ |
| 2 | 点击“读取” | 显示固件信息和分区清单 |
| 3 | 勾选需要的分区 | 初始状态全部不勾选 |
| 4 | 选择目录与线程数 | 可输入 1～64，默认 4；低配建议 1～2，TGZ 固定顺序扫描 |
| 5 | 点击“提取所选分区” | 输出为 `分区名.img` |

增量 OTA 中标记为“需旧镜像”的分区，需要在“旧镜像目录”中放置同名文件，例如 `system.img`。

## ⚙️ 在线 ZIP 按需模式

在线模式的工作流程：

1. 通过 Range 读取 ZIP 尾部中央目录
2. 找到 `payload.bin` 时读取 Payload 清单，并按操作请求所选分区数据
3. 没有 `payload.bin` 时自动切换通用 ZIP 模式，列出包内 `.img`
4. 通用 ZIP 提取时只请求所选条目的压缩数据，支持 Store 与 Deflate
5. 使用有上限的 4 MiB 分块缓存，避免重复请求

通用 ZIP 切换时日志会明确显示：

```text
检测到远程 ZIP 不包含 payload.bin，直接按通用 ZIP 模式读取。
ZIP 中共 385 个文件条目，识别到 99 个镜像分区。
```

> [!NOTE]
>
> 在线地址必须正确返回 `206 Partial Content`。OTA ZIP 内的 `payload.bin` 还必须是 Store（未二次压缩）。条件不满足时程序会提示改用本地文件；用于显示机型和版本的小型 metadata 文件同样按需读取。

## 📦 TGZ 在线模式说明

TGZ 是 gzip 串流格式，没有 ZIP 中央目录，无法从文件尾直接定位某个镜像，也无法只下载位于中间的一段后独立解压。因此在线 TGZ 会采用以下流程：

1. 点击“读取”后先弹窗说明需要完整缓存，并由用户确认
2. 只下载一次到系统临时目录，同时显示字节进度并支持快速取消
3. 扫描清单并提取时复用同一个缓存，不会再次下载
4. 换包或正常退出程序时自动删除缓存；取消读取时也会立即清理

这个限制仅适用于 TGZ / TAR.GZ。在线 OTA ZIP 和通用 ZIP 仍然严格使用 Range 按需读取。

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

如需验证真实在线包，可设置 `ONLINE_OTA_URL`、`GENERIC_ZIP_URL` 或 `TGZ_PREFIX_URL`，再运行名称中带 `FromEnvironment` 的对应集成测试。TGZ 前缀测试最多请求 4 MiB。

## 🎉 致谢

- [ssut/payload-dumper-go](https://github.com/ssut/payload-dumper-go) — Payload 解析与提取核心
- [Android Open Source Project](https://source.android.com/) — Payload update metadata 定义
- 致每一位提交 Issue、测试固件和改进建议的用户

## 📄 许可

本项目自身代码采用 [MIT License](LICENSE)。Payload 核心基于 Apache-2.0 许可的 `payload-dumper-go`，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) 和 `third_party/payload-dumper-go/LICENSE`。
