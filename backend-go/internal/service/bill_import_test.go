package service

import (
	"archive/zip"
	"bytes"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"pennypickbackend/internal/model"
)

// 微信 CSV 样例（表头 + 数据行），含正常支出/收入、退款、备注。
const wechatCSV = `微信支付账单明细
----------------------微信支付账单明细列表--------------------
交易时间,交易类型,交易对方,商品,收/支,金额(元),支付方式,当前状态,交易单号,商户单号,备注
2026-08-25 17:42:04,商户消费,机关食堂,机关食堂,支出,20,招商银行信用卡(3269),支付成功,45000001,,/
2026-08-23 18:41:28,扫二维码付款,刘加凤,收款方备注:二维码收款,支出,12,招商银行储蓄卡(5488),已转账,53110002,,
2026-08-26 02:00:00,商户消费,某店铺,商品描述,收入,50,零钱,支付成功,45000002,,退款单
2026-07-26 10:14:43,商户消费,华住酒店,酒店住宿,支出,140,招商银行信用卡(3269),已全额退款,45000003,,
`

func TestParseWechatCSV(t *testing.T) {
	svc := NewBillImportService(nil)
	items, err := svc.parseWechat([]byte(wechatCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("期望 4 条，实际 %d", len(items))
	}
	normal, filtered := 0, 0
	for _, it := range items {
		if it.IsFiltered {
			filtered++
			if it.FilterReason == "" {
				t.Error("过滤项缺少原因")
			}
			continue
		}
		normal++
	}
	if normal != 3 {
		t.Errorf("期望 3 条正常记录，实际 %d", normal)
	}
	if filtered != 1 {
		t.Errorf("期望过滤 1 条（已全额退款），实际 %d", filtered)
	}
	// 第一笔备注：商品 + 备注（备注为 / 时忽略）
	if items[0].Note != "机关食堂" {
		t.Errorf("备注拼接错误: %q", items[0].Note)
	}
	// 第三笔：商品 + 备注
	if items[2].Note != "商品描述 / 退款单" {
		t.Errorf("备注拼接错误: %q", items[2].Note)
	}
	// 退款项：交易单号保留
	if items[3].PlatformOrderNo != "45000003" {
		t.Errorf("退款项订单号丢失: %q", items[3].PlatformOrderNo)
	}
}

func TestParseAlipayCSV(t *testing.T) {
	csv := `------------------------------------------------------------------------------------
导出信息：
姓名：测试
------------------------支付宝支付科技有限公司  电子客户回单------------------------
交易时间,交易分类,交易对方,对方账号,商品说明,收/支,金额,收/付款方式,交易状态,交易订单号,商家订单号,备注,
2026-08-26 02:25:37,投资理财,华泰紫金货币增利E,/,余额宝-收益发放,不计收支,4.85,余额宝,交易成功,20260826001,,
2026-08-23 17:38:03,商业服务,山东高速,/,ETC通行费,支出,40.85,招商银行信用卡(3269),交易成功,20260826002,,
2026-08-22 02:13:27,投资理财,华泰紫金货币增利E,/,余额宝-收益发放,不计收支,4.80,余额宝,交易成功,20260826003,,
2026-08-21 19:30:04,商业服务,某餐饮公司,/,午餐,支出,36.00,余额,交易成功,20260826004,,公司团建
2026-08-20 02:02:41,转账红包,某人,/,红包,收入,200.00,余额,交易成功,20260826005,,
`
	svc := NewBillImportService(nil)
	items, err := svc.parseAlipay([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("期望 5 条，实际 %d", len(items))
	}
	exp, inc, flt := 0, 0, 0
	for _, it := range items {
		if it.IsFiltered {
			flt++
			continue
		}
		if it.Type == model.TypeExpense {
			exp++
		} else if it.Type == model.TypeIncome {
			inc++
		}
	}
	if exp != 2 || inc != 1 || flt != 2 {
		t.Errorf("期望 支出2 收入1 过滤2，实际 支出%d 收入%d 过滤%d", exp, inc, flt)
	}
	// 备注拼接：商品说明 + 备注
	for _, it := range items {
		if it.PlatformOrderNo == "20260826004" {
			if it.Note != "午餐 / 公司团建" {
				t.Errorf("支付宝备注拼接错误: %q", it.Note)
			}
		}
	}
	// 编码兼容：GBK 编码也应可解析
	items2, err := svc.parseAlipay(encodeGBK(csv))
	if err != nil {
		t.Fatalf("GBK 解析失败: %v", err)
	}
	if len(items2) != 5 {
		t.Errorf("GBK 解析期望 5 条，实际 %d", len(items2))
	}
}

// TestParseWechatXLSX 构造微信 xlsx（zip + sharedStrings + sheet1）验证标准 xlsx 解析。
func TestParseWechatXLSX(t *testing.T) {
	shared := []string{
		"交易时间", "交易类型", "交易对方", "商品", "收/支", "金额(元)", "支付方式", "当前状态", "交易单号", "商户单号", "备注",
		"商户消费", "机关食堂", "支出", "招商银行信用卡(3269)", "支付成功", "45000001", "/",
		"收入", "零钱", "45000002", "商品描述", "退款单",
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range shared {
		sb.WriteString("<si><t>" + s + "</t></si>")
	}
	sb.WriteString("</sst>")

	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	// 表头行
	sheet.WriteString(`<row r="1">`)
	headers := []string{"A1", "B1", "C1", "D1", "E1", "F1", "G1", "H1", "I1", "J1", "K1"}
	for i, ref := range headers {
		sheet.WriteString(`<c r="` + ref + `" t="s"><v>` + itoa(i) + `</v></c>`)
	}
	sheet.WriteString(`</row>`)
	// 数据行：正常支出（备注为 /）
	sheet.WriteString(`<row r="2"><c r="A2"><v>2026-08-25 17:42:04</v></c><c r="B2" t="s"><v>11</v></c><c r="C2" t="s"><v>12</v></c><c r="D2" t="s"><v>12</v></c><c r="E2" t="s"><v>13</v></c><c r="F2"><v>20</v></c><c r="G2" t="s"><v>14</v></c><c r="H2" t="s"><v>15</v></c><c r="I2" t="s"><v>16</v></c><c r="J2"><v></v></c><c r="K2" t="s"><v>17</v></c></row>`)
	// 数据行：收入带备注（D=商品21 商品描述, I=单号20 45000002, K=备注22 退款单）
	sheet.WriteString(`<row r="3"><c r="A3"><v>2026-08-26 02:00:00</v></c><c r="B3" t="s"><v>11</v></c><c r="C3" t="s"><v>12</v></c><c r="D3" t="s"><v>21</v></c><c r="E3" t="s"><v>18</v></c><c r="F3"><v>50</v></c><c r="G3" t="s"><v>19</v></c><c r="H3" t="s"><v>15</v></c><c r="I3" t="s"><v>20</v></c><c r="J3"><v></v></c><c r="K3" t="s"><v>22</v></c></row>`)
	sheet.WriteString(`</sheetData></worksheet>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"xl/sharedStrings.xml":     sb.String(),
		"xl/worksheets/sheet1.xml": sheet.String(),
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()

	svc := NewBillImportService(nil)
	items, err := svc.parseWechat(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(items))
	}
	if items[0].Note != "机关食堂" {
		t.Errorf("xlsx 备注错误: %q", items[0].Note)
	}
	if items[0].Type != model.TypeExpense || items[0].Amount != 20 {
		t.Errorf("xlsx 支出解析错误: type=%s amount=%v", items[0].Type, items[0].Amount)
	}
	if items[1].Type != model.TypeIncome || items[1].Note != "商品描述 / 退款单" {
		t.Errorf("xlsx 收入/备注错误: type=%s note=%q", items[1].Type, items[1].Note)
	}
}

func TestBuildNoteWithRemark(t *testing.T) {
	// 支付宝表头行映射：交易时间,交易分类,交易对方,对方账号,商品说明,收/支,金额,收/付款方式,交易状态,交易订单号,商家订单号,备注
	h := map[string]int{"time": 0, "trade_type": 1, "counterparty": 2, "product": 4, "flow": 5, "amount": 6, "pay_way": 7, "status": 8, "order_no": 9, "note": 11}
	row := []string{"2026-08-26 02:25:37", "商业服务", "某餐饮公司", "/", "午餐", "支出", "36.00", "招商银行信用卡(3269)", "交易成功", "20260826xxx", "", "公司团建聚餐"}
	it, ok := buildAlipayItem(h, row)
	if !ok {
		t.Fatal("buildAlipayItem 返回 ok=false")
	}
	if it.IsFiltered {
		t.Fatalf("不应被过滤: %s", it.FilterReason)
	}
	want := "午餐 / 公司团建聚餐"
	if it.Note != want {
		t.Errorf("支付宝备注拼接错误: 期望 %q 实际 %q", want, it.Note)
	}
	// 备注为空时仅保留商品说明
	row[11] = ""
	it, _ = buildAlipayItem(h, row)
	if it.Note != "午餐" {
		t.Errorf("支付宝无备注时应仅保留商品说明, 实际 %q", it.Note)
	}
	// 微信：商品 + 备注（备注为 / 时忽略）
	hw := map[string]int{"time": 0, "trade_type": 1, "counterparty": 2, "product": 3, "flow": 4, "amount": 5, "pay_way": 6, "status": 7, "order_no": 8, "note": 10}
	wrow := []string{"2026-08-26 17:42:04", "商户消费", "机关食堂", "机关食堂", "支出", "20", "招商银行信用卡(3269)", "支付成功", "4500000xxx", "商户单号", "/"}
	itw, ok := buildWechatItem(hw, wrow)
	if !ok {
		t.Fatal("buildWechatItem 返回 ok=false")
	}
	if itw.Note != "机关食堂" {
		t.Errorf("微信备注为/时应忽略, 实际 %q", itw.Note)
	}
	wrow[10] = "员工餐补"
	itw, _ = buildWechatItem(hw, wrow)
	if itw.Note != "机关食堂 / 员工餐补" {
		t.Errorf("微信备注拼接错误: 实际 %q", itw.Note)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// encodeGBK 将 UTF-8 文本编码为 GBK 字节（用于编码兼容测试）。
func encodeGBK(s string) []byte {
	enc := simplifiedchinese.GBK.NewEncoder()
	out, _, err := transform.String(enc, s)
	if err != nil {
		return []byte(s)
	}
	return []byte(out)
}
