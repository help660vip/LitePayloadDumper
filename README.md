# LitePayloadDumper

这是一个 Windows 下用的 Android 固件分区提取工具。输入本地固件或在线地址，读取分区表后，勾选要导出的镜像即可。

## 下载

在 [Releases](https://github.com/help660vip/LitePayloadDumper/releases) 里有两个版本：

| 文件 | 系统 |
| --- | --- |
| `LitePayloadDumper_Win7.exe` | Windows 7 SP1 64 位 |
| `LitePayloadDumper_Win10-11.exe` | Windows 10 / 11 64 位 |

都是单文件 EXE，不用安装 .NET、Java、Python、VC++ 运行库，也没有需要放在旁边的 DLL。

## 怎么用

1. 选择本地固件，或粘贴 HTTP / HTTPS 链接。
2. 点击“读取”，等待分区列表出现。
3. 用搜索框找分区，勾选需要的项目。刚读取完成时不会自动勾选任何分区。
4. 确认保存目录和线程数。默认 4 线程，可填 1～64。
5. 点击“提取所选分区”。

读取成功后，界面会列出机型、设备代号、Android 版本、系统版本、安全补丁日期和构建日期等信息。

在线地址的默认保存位置取 Windows 记录的“下载”文件夹。如果这个文件夹在 C 盘，或者程序无法读取它，则改用 EXE 所在目录。本地固件仍默认保存到固件旁边。

## 能读哪些包

- 含 `payload.bin` 的 OTA ZIP
- 独立 `payload.bin`
- 直接包含 `.img` 的线刷 ZIP
- `.tgz` / `.tar.gz` 线刷包

在线 ZIP 使用 HTTP Range，只读取 ZIP 目录、Payload 清单和所选分区用到的字节段。远程 ZIP 没有 `payload.bin` 时，程序会改按普通 ZIP 查找其中的 `.img`。如果服务器不支持 Range，程序会报错，不会悄悄下载整个 ZIP。

TGZ 不能像 ZIP 那样随机读取单个文件。打开在线 TGZ 时，程序会先询问是否完整缓存；缓存会供后续提取复用，换包或退出时清理。缓存过程中取消也会立即清理。

## 已知限制

- 只支持 64 位 Windows。Windows 7 需要 SP1。
- 增量 OTA 中依赖旧镜像的分区无法直接还原，请换用完整包。
- 在线文件所在服务器必须允许 HTTP Range 请求。

## 编译

Windows 7 版本使用 Go 1.20.14；Windows 10 / 11 版本使用当前稳定版 Go：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 `
  -Go120 'C:\Go120\bin\go.exe' `
  -GoModern 'C:\Go\bin\go.exe'
```

测试：

```powershell
go test ./...
```

## 许可

Payload 解析核心基于 [ssut/payload-dumper-go](https://github.com/ssut/payload-dumper-go)。项目代码采用 [MIT License](LICENSE)，第三方许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
