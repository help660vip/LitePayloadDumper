## LitePayloadDumper v1.0.0

首个公开版本，支持 Android OTA Payload 与常见线刷包分区查看、信息识别和提取。

### 主要功能

- 支持本地 OTA ZIP、`payload.bin`、通用线刷 ZIP、TGZ / TAR.GZ
- 在线模式使用 HTTP Range，只读取 ZIP 目录、Payload 清单与所选分区的数据段
- 远程 ZIP 无 `payload.bin` 时自动进入通用 ZIP 模式，列出并按需提取包内 `.img`
- 通用 ZIP 支持 Store / Deflate，只请求所选镜像的压缩数据
- 在线 TGZ 在用户确认后完整缓存一次；提取复用缓存，取消、换包或退出时清理
- 显示机型、设备代号、安卓版本、系统版本、安全补丁日期、SDK 和构建日期
- 分区默认全不选，支持常用启动分区、全选和全不选
- 支持按分区名称实时搜索，并保留搜索前后的勾选状态
- Payload / ZIP 线程数支持用户输入 1～64，默认 4 线程；TGZ 按单一串流顺序扫描
- 字节级实时进度，连接、提取、校验阶段均有明确提示
- 下载、解压、增量补丁、校验过程均支持快速取消，并清理未完成镜像
- Win7 与 Win10/11 分开构建，均为 64 位单文件 EXE，无需安装任何框架或运行库

### 下载选择

- Windows 7 SP1：`LitePayloadDumper_Win7.exe`
- Windows 10 / 11：`LitePayloadDumper_Win10-11.exe`

可使用随 Release 提供的 `SHA256SUMS.txt` 校验文件完整性。

> 在线 ZIP 服务器必须支持 HTTP Range，且 OTA ZIP 内的 `payload.bin` 必须未二次压缩。否则程序会停止并提示使用本地文件，不会自动下载完整 ZIP。TGZ 因 gzip 格式无法随机访问，程序会在完整缓存前单独征求确认。
