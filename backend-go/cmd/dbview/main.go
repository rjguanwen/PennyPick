// Command dbview 是拾财数据库运维解密工具。
//
// 功能：
//   -out  解密为明文 SQLite 文件，供 Navicat/DBeaver 等客户端查看；
//   -dump 直接把内容导出为 SQL 脚本到 stdout，全程不落明文 db 文件。
//
// 密码交互式输入，不会进入命令行历史与进程列表。
package main

import (
	"bufio"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	_ "github.com/glebarez/sqlite" // 注册名为 "sqlite" 的 database/sql 驱动
	"golang.org/x/term"

	"pennypickbackend/internal/crypto"
)

func main() {
	in := flag.String("file", "", "加密数据库文件路径（.enc）")
	out := flag.String("out", "", "解密输出路径（明文 .db，供 Navicat/DBeaver 查看）")
	dump := flag.Bool("dump", false, "直接导出 SQL 脚本到 stdout，不落明文 db 文件")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "拾财数据库运维解密工具\n\n")
		fmt.Fprintf(os.Stderr, "用法: pennypick-dbview -file <enc> [-out <plain.db> | -dump]\n")
		fmt.Fprintf(os.Stderr, "  -file  加密数据库文件（.enc）\n")
		fmt.Fprintf(os.Stderr, "  -out   解密输出为明文 SQLite 文件，可用 Navicat/DBeaver 打开\n")
		fmt.Fprintf(os.Stderr, "  -dump  直接输出 SQL 脚本到 stdout，不落明文文件\n")
		fmt.Fprintf(os.Stderr, "示例: pennypick-dbview -file pennypick.db.enc -out view.db\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *in == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *out == "" && !*dump {
		fmt.Fprintln(os.Stderr, "错误：必须指定 -out 或 -dump 之一")
		flag.Usage()
		os.Exit(2)
	}

	pass := promptPassphrase()

	enc, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取加密文件失败: %v\n", err)
		os.Exit(1)
	}
	plain, err := crypto.DecryptBytes(enc, pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解密失败: %v\n", err)
		os.Exit(1)
	}

	if *dump {
		if err := dumpSQL(plain, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "导出 SQL 失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := os.WriteFile(*out, plain, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "写出解密文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已解密到 %s\n请用 Navicat/DBeaver 打开查看，查看完请删除该明文文件。\n", *out)
}

// stdinReader 复用同一 reader，避免 bufio 预读缓冲吞掉后续行。
var stdinReader *bufio.Reader

// promptPassphrase 读取主密码：终端下交互式不回显输入，非终端（管道/脚本）从 stdin 读一行。
func promptPassphrase() string {
	var pass string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print("请输入数据库主密码: ")
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

// dumpSQL 将解密后的明文 SQLite 内容导出为 SQL 脚本。
func dumpSQL(plain []byte, out io.Writer) error {
	tmp, err := writeTempPlain(plain)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}

	// DDL 与表清单
	rows, err := db.Query(`SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		ORDER BY rowid`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var typ, name, sqlText string
		if err := rows.Scan(&typ, &name, &sqlText); err != nil {
			return err
		}
		fmt.Fprintf(out, "DROP %s IF EXISTS %s;\n", strings.ToUpper(typ), quoteIdent(name))
		fmt.Fprintf(out, "%s;\n", sqlText)
		if typ == "table" {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, t := range tables {
		if err := dumpTableData(db, out, t); err != nil {
			return err
		}
	}
	return nil
}

func dumpTableData(db *sql.DB, out io.Writer, table string) error {
	rows, err := db.Query("SELECT * FROM " + quoteIdent(table))
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	var sb strings.Builder
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		sb.Reset()
		sb.WriteString("INSERT INTO ")
		sb.WriteString(quoteIdent(table))
		sb.WriteString(" VALUES (")
		for i, v := range vals {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(sqlLiteral(v))
		}
		sb.WriteString(");\n")
		if _, err := io.WriteString(out, sb.String()); err != nil {
			return err
		}
	}
	return rows.Err()
}

// sqlLiteral 将驱动返回值转义为 SQL 字面量。
func sqlLiteral(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		return "X'" + hex.EncodeToString(val) + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''") + "'"
	}
}

// quoteIdent 按 SQLite 规则转义标识符。
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func writeTempPlain(data []byte) (string, error) {
	f, err := os.CreateTemp(os.TempDir(), "pennypick-dump-*.db")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	_ = os.Chmod(name, 0o600)
	return name, nil
}
