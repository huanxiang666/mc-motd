// mc-motd — Minecraft 服务器 MOTD 查询工具（跨平台单文件）
// 支持 Java 版（Server List Ping，1.7+）和基岩版（RakNet Unconnected Ping）。
// 纯标准库，零第三方依赖。交叉编译出 Windows / Linux / macOS 单文件。
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultJavaPort = 25565
	defaultBEPort   = 19132
	defaultTimeout  = 3 * time.Second
	// 仅用于查询的协议版本号，不影响兼容性
	protocolVersion = 767
)

// raknetMagic 是 RakNet 协议的固定魔数
var raknetMagic = []byte{0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD, 0x12, 0x34, 0x56, 0x78}

// ============================ VarInt ============================

func writeVarint(v int) []byte {
	var out []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			out = append(out, b|0x80)
		} else {
			out = append(out, b)
			return out
		}
	}
}

type byteReader struct {
	conn net.Conn
}

func (r byteReader) ReadByte() (byte, error) {
	buf := make([]byte, 1)
	_, err := r.conn.Read(buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func readVarint(r byteReader) (int, error) {
	num := 0
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		num |= int(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return num, nil
		}
	}
	return 0, errors.New("VarInt 过长")
}

// ============================ Java 版 ============================

// JavaStatus 是 Java 版服务器状态响应（只取关心字段，其余原样保留在 Raw）
type JavaStatus struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
	Description json.RawMessage `json:"description"`
	Raw         map[string]interface{}
}

// motdText 把 description（可能是字符串或带 extra 的对象）拍平成纯文本
func motdText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	var sb strings.Builder
	if t, ok := obj["text"].(string); ok {
		sb.WriteString(t)
	}
	if extra, ok := obj["extra"].([]interface{}); ok {
		for _, e := range extra {
			if m, ok := e.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			} else if t, ok := e.(string); ok {
				sb.WriteString(t)
			}
		}
	}
	return sb.String()
}

// QueryJava 查询 Java 版服务器，返回状态 JSON 的 map
func QueryJava(host string, port int, timeout time.Duration) (map[string]interface{}, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), timeout)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Handshake
	var handshake []byte
	handshake = append(handshake, 0x00) // packet id
	handshake = append(handshake, writeVarint(protocolVersion)...)
	hostBytes := []byte(host)
	handshake = append(handshake, writeVarint(len(hostBytes))...)
	handshake = append(handshake, hostBytes...)
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], uint16(port))
	handshake = append(handshake, portBuf[:]...)
	handshake = append(handshake, writeVarint(1)...) // next state: status

	packet := append(writeVarint(len(handshake)), handshake...)
	if _, err := conn.Write(packet); err != nil {
		return nil, err
	}
	// Status Request
	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return nil, err
	}

	br := byteReader{conn}
	if _, err := readVarint(br); err != nil { // 包总长
		return nil, err
	}
	pktID, err := readVarint(br) // 应为 0x00
	if err != nil {
		return nil, err
	}
	if pktID != 0 {
		return nil, fmt.Errorf("意外的响应包 id: %d", pktID)
	}
	jsonLen, err := readVarint(br)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, jsonLen)
	if _, err := readFull(conn, payload); err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return result, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// JavaSummary 整理成可读字符串
func JavaSummary(raw map[string]interface{}) string {
	var js JavaStatus
	b, _ := json.Marshal(raw)
	json.Unmarshal(b, &js)
	js.Raw = raw
	motd := motdText(js.Description)
	return fmt.Sprintf("%s | 在线 %d/%d | 版本 %s",
		motd, js.Players.Online, js.Players.Max, js.Version.Name)
}

// ============================ 基岩版 ============================

// BedrockStatus 是基岩版 pong 的结构化解析
type BedrockStatus struct {
	Edition   string `json:"edition"`
	Motd      string `json:"motd"`
	Protocol  string `json:"protocol"`
	Version   string `json:"version"`
	Online    string `json:"online"`
	Max       string `json:"max"`
	MotdLine2 string `json:"motd_line2"`
}

// QueryBedrock 查询基岩版服务器（UDP）
func QueryBedrock(host string, port int, timeout time.Duration) (*BedrockStatus, error) {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Unconnected Ping: 0x01 + 时间戳(8) + magic(16) + client guid(8)
	ping := make([]byte, 0, 33)
	ping = append(ping, 0x01)
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(time.Now().Unix()))
	ping = append(ping, ts[:]...)
	ping = append(ping, raknetMagic...)
	ping = append(ping, make([]byte, 8)...) // client guid，任意

	if _, err := conn.Write(ping); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	data := buf[:n]
	if len(data) == 0 || data[0] != 0x1C {
		return nil, errors.New("不是有效的基岩版 pong 响应")
	}
	// 跳过: 0x1C + 时间戳(8) + server guid(8) + magic(16) + 长度(2)
	offset := 1 + 8 + 8 + 16
	if len(data) < offset+2 {
		return nil, errors.New("pong 数据过短")
	}
	strlen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	if len(data) < offset+2+strlen {
		return nil, errors.New("pong 字符串数据不完整")
	}
	text := string(data[offset+2 : offset+2+strlen])
	parts := strings.Split(text, ";")
	field := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	return &BedrockStatus{
		Edition:   field(0),
		Motd:      field(1),
		Protocol:  field(2),
		Version:   field(3),
		Online:    field(4),
		Max:       field(5),
		MotdLine2: field(7),
	}, nil
}

func (b *BedrockStatus) Summary() string {
	return fmt.Sprintf("%s | 在线 %s/%s | 版本 %s | %s",
		b.Motd, b.Online, b.Max, b.Version, b.Edition)
}

// ============================ 主流程 ============================

func resolveHost(host string) (string, error) {
	if net.ParseIP(host) != nil {
		return host, nil
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}

func main() {
	// 手动解析参数（flag 包遇到非 flag 参数就停，"host -p 25599" 这种写法 flag 会失效）
	port := 0
	edition := "je"
	timeout := defaultTimeout
	jsonOut := false
	host := ""

	usage := func() {
		fmt.Fprintln(os.Stderr, "mc-motd — Minecraft 服务器 MOTD 查询")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "用法: mc-motd [选项] <服务器IP或域名>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "选项:")
		fmt.Fprintln(os.Stderr, "  -e je|be     版本（默认 je=Java）")
		fmt.Fprintln(os.Stderr, "  -p 端口      Java 默认 25565，基岩默认 19132")
		fmt.Fprintln(os.Stderr, "  -t 3s        超时")
		fmt.Fprintln(os.Stderr, "  -json        输出原始 JSON")
		fmt.Fprintln(os.Stderr, "  -h           显示帮助")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "示例:")
		fmt.Fprintln(os.Stderr, "  mc-motd mc.hypixel.net           查 Java 版")
		fmt.Fprintln(os.Stderr, "  mc-motd -e be play.example.com   查基岩版")
		fmt.Fprintln(os.Stderr, "  mc-motd -p 25566 -json 1.2.3.4   指定端口+JSON输出")
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "-h" || a == "--help":
			usage()
			return
		case a == "-json" || a == "--json":
			jsonOut = true
		case a == "-e" || a == "--edition":
			edition = next()
		case strings.HasPrefix(a, "-e="):
			edition = a[3:]
		case a == "-p" || a == "--port":
			fmt.Sscanf(next(), "%d", &port)
		case strings.HasPrefix(a, "-p="):
			fmt.Sscanf(a[3:], "%d", &port)
		case a == "-t" || a == "--timeout":
			if d, err := time.ParseDuration(next()); err == nil {
				timeout = d
			}
		case strings.HasPrefix(a, "-t="):
			if d, err := time.ParseDuration(a[3:]); err == nil {
				timeout = d
			}
		case !strings.HasPrefix(a, "-"):
			if host == "" {
				host = a
			}
		}
	}

	if host == "" {
		usage()
		os.Exit(2)
	}

	ip, err := resolveHost(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[失败] 无法解析 %s: %v\n", host, err)
		os.Exit(1)
	}

	if edition == "be" {
		p := port
		if p == 0 {
			p = defaultBEPort
		}
		st, err := QueryBedrock(ip, p, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[失败] %v\n", err)
			os.Exit(1)
		}
		if jsonOut {
			b, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Println(st.Summary())
		}
	} else {
		p := port
		if p == 0 {
			p = defaultJavaPort
		}
		raw, err := QueryJava(ip, p, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[失败] %v\n", err)
			os.Exit(1)
		}
		if jsonOut {
			b, _ := json.MarshalIndent(raw, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Println(JavaSummary(raw))
		}
	}
}
