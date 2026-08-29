## LitePayloadDumper v1.0.2

- 修复固件信息中的四字标签被截断
- 在线地址优先使用 Windows 配置的下载文件夹；该目录位于 C 盘或无法读取时，改用 EXE 所在目录
- 标题栏增加 GitHub 按钮，可直接打开项目主页
- README 改为直接的下载、操作和限制说明
- Logo 继续内嵌在 EXE 中，仓库不再保存原图

### 下载

- Windows 7 SP1：`LitePayloadDumper_Win7.exe`
- Windows 10 / 11：`LitePayloadDumper_Win10-11.exe`

两个版本均为 64 位单文件 EXE，无需安装外部框架或运行库。可使用 `SHA256SUMS.txt` 校验文件完整性。
