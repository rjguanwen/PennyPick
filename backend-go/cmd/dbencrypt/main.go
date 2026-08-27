// Command dbencrypt 是拾财数据库加密运维工具。
//
// 支持三种模式：
//   encrypt  把明文 SQLite 库加密为 .enc
//   decrypt  把 .enc 解密为明文 SQLite 库（与 dbview 的 -out 等价）
//   chpass   修改 .enc 的主密码
//
// 密码交互式输入（非终端环境从 stdin 读一行），不会进入命令行历史。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"pennypickbackend/internal/crypto"
)

func main() {
	mode := flag.String("mode", "", "操作模式：encrypt / decrypt / chpass")
	in := flag.String("in", "", "输入文件路径")
	out := flag.String("out", "", "输出文件路径（encrypt/decrypt 必填）")
	file := flag.String("file", "", ".enc 文件路径（chpass 必填，等价于 -in）")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "拾财数据库加密运维工具\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n")
		fmt.Fprintf(os.Stderr, "  dbencrypt -mode encrypt -in plain.db -out plain.db.enc\n")
		fmt.Fprintf(os.Stderr, "  dbencrypt -mode decrypt -in plain.db.enc -out plain.db\n")
		fmt.Fprintf(os.Stderr, "  dbencrypt -mode chpass -file plain.db.enc\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	switch *mode {
	case "encrypt":
		if *in == "" || *out == "" {
			flag.Usage()
			os.Exit(2)
		}
		newPass := promptPassphrase("请设置新主密码")
		confirm := promptPassphrase("请再次输入新主密码")
		if newPass != confirm {
			fmt.Fprintln(os.Stderr, "错误：两次输入的密码不一致")
			os.Exit(1)
		}
		if err := crypto.EncryptFile(*in, *out, newPass); err != nil {
			fmt.Fprintf(os.Stderr, "加密失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已加密：%s → %s\n", *in, *out)
	case "decrypt":
		if *in == "" || *out == "" {
			flag.Usage()
			os.Exit(2)
		}
		plain, err := crypto.DecryptFile(*in, promptPassphrase("请输入主密码"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "解密失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*out, plain, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "写出失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已解密：%s → %s\n", *in, *out)
	case "chpass":
		target := *file
		if target == "" {
			target = *in
		}
		if target == "" {
			flag.Usage()
			os.Exit(2)
		}
		oldPass := promptPassphrase("请输入当前主密码")
		plain, err := crypto.DecryptFile(target, oldPass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "解密失败（当前密码可能错误）: %v\n", err)
			os.Exit(1)
		}
		newPass := promptPassphrase("请设置新主密码")
		confirm := promptPassphrase("请再次输入新主密码")
		if newPass != confirm {
			fmt.Fprintln(os.Stderr, "错误：两次输入的密码不一致")
			os.Exit(1)
		}
		enc, err := crypto.EncryptBytes(plain, newPass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加密失败: %v\n", err)
			os.Exit(1)
		}
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, enc, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "写入临时文件失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.Rename(tmp, target); err != nil {
			_ = os.Remove(tmp)
			fmt.Fprintf(os.Stderr, "替换文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已修改主密码：%s\n", target)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

// stdinReader 复用同一 reader，避免 bufio 预读缓冲吞掉后续行。
var stdinReader *bufio.Reader

// promptPassphrase 读取主密码：终端下交互式不回显，非终端从 stdin 读一行。
func promptPassphrase(label string) string {
	var pass string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print(label + ": ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取密码失败: %v\n", err)
			os.Exit(1)
		}
		pass = string(raw)
	} else {
		if stdinReader == nil {
			stdinReader = bufio.NewReader(os.Stdin)
		}
		line, err := stdinReader.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "读取密码失败: %v\n", err)
			os.Exit(1)
		}
		pass = strings.TrimRight(line, "\r\n")
	}
	if pass == "" {
		fmt.Fprintln(os.Stderr, "错误：密码不能为空")
		os.Exit(1)
	}
	return pass
}
