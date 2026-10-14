# mc-motd

Minecraft 服务器 MOTD 查询工具 — **单文件可执行**，支持 Java 版和基岩版，跨 Windows / Linux / macOS。

[English](#english) · [中文](#中文)

---

## 中文

### 特性

- **Java 版**：现代 Server List Ping 协议（1.7+ 全部服务器），返回 MOTD、在线人数、版本、图标等完整 JSON
- **基岩版**：RakNet Unconnected Ping（UDP），解析 MOTD、在线、版本、端口
- **单文件**：Go 编译，无需安装运行时，双击即用
- **跨平台**：Windows / Linux / macOS（x64 和 ARM64）
- **纯标准库**：零第三方依赖，编译干净

### 下载

到 [Releases](../../releases) 下载对应平台的单文件：

| 文件 | 平台 |
|---|---|
| `mc-motd-windows-amd64.exe` | Windows 64位 |
| `mc-motd-linux-amd64` | Linux 64位 |
| `mc-motd-macos-amd64` | macOS Intel |
| `mc-motd-macos-arm64` | macOS Apple Silicon |

### 用法

```bash
# 查 Java 版（默认端口 25565）
mc-motd mc.hypixel.net

# 查基岩版（默认端口 19132）
mc-motd -e be play.example.com

# 指定端口 + JSON 输出
mc-motd -p 25566 -json 1.2.3.4
```

### 参数

| 参数 | 说明 | 默认 |
|---|---|---|
| `-e je\|be` | 版本：je=Java，be=基岩 | `je` |
| `-p 端口` | 服务器端口 | Java 25565 / 基岩 19132 |
| `-t 3s` | 超时时间 | `3s` |
| `-json` | 输出原始 JSON | 关闭 |
| `-h` | 帮助 | - |

### 从源码构建

需要 Go 1.20+：

```bash
git clone https://github.com/huanxiang666/mc-motd.git
cd mc-motd
go build -o mc-motd .
```

交叉编译（无需目标平台环境）：

```bash
GOOS=windows GOARCH=amd64 go build -o mc-motd.exe .
GOOS=linux   GOARCH=amd64 go build -o mc-motd .
GOOS=darwin  GOARCH=arm64 go build -o mc-motd .
```

### 协议说明

- **Java 版**：先查 `_minecraft._tcp.<host>` 的 SRV 记录，命中则用 SRV 的 target:port（未手动指定端口时）；再发 VarInt 握手包 + Status Request → 接收 JSON 状态响应。协议版本号固定填 767，仅用于查询。
- **基岩版**：不查 SRV，直接发 `0x01` Unconnected Ping + 时间戳 + magic + GUID → 解析 pong 的分号分隔字段。

### 协议

[MIT](LICENSE)

---

## English

Query Minecraft server MOTD / player count / version — **single-file binary**, supports both Java Edition and Bedrock Edition, cross-platform (Windows / Linux / macOS).

### Features

- **Java Edition**: modern Server List Ping (1.7+), full JSON status response
- **Bedrock Edition**: RakNet Unconnected Ping (UDP), structured MOTD/players/version parsing
- **Single binary**: no runtime required, double-click to run
- **Cross-platform**: Windows / Linux / macOS (x64 & ARM64)
- **Pure standard library**: zero third-party dependencies

### Usage

```bash
# Java Edition (default port 25565)
mc-motd mc.hypixel.net

# Bedrock Edition (default port 19132)
mc-motd -e be play.example.com

# Custom port + JSON output
mc-motd -p 25566 -json 1.2.3.4
```

### Build

Requires Go 1.20+:

```bash
git clone https://github.com/huanxiang666/mc-motd.git
cd mc-motd
go build -o mc-motd .
```

### License

[MIT](LICENSE)
