package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

// wealthKeywords 理财/投资类关键词：命中则视为不计入收支的记录，不导入。
var wealthKeywords = []string{
	"零钱通", "理财通", "余额宝", "基金", "理财", "定投", "保险", "黄金",
	"朝朝盈", "朝朝宝", "银行存款", "国债", "收益发放", "股票", "投资",
}

// BillImportService 账单导入服务：解析 + 去重检测 + 确认落库。
type BillImportService struct {
	db *gorm.DB
}

func NewBillImportService(db *gorm.DB) *BillImportService {
	return &BillImportService{db: db}
}

// ParseRequest 解析请求。
type ParseRequest struct {
	Platform string
	FileName string
	Data     []byte
}

// ParseResult 复用 model.ParseResult。
type ParseResult = model.ParseResult

// Parse 解析账单文件并做去重检测。纯读操作，不写库。
func (s *BillImportService) Parse(req ParseRequest, userID uint) (*ParseResult, error) {
	var items []model.ImportItem
	var err error
	switch req.Platform {
	case model.PlatformWechat:
		items, err = s.parseWechat(req.Data)
	case model.PlatformAlipay:
		items, err = s.parseAlipay(req.Data)
	default:
		return nil, errors.New("不支持的平台")
	}
	if err != nil {
		return nil, err
	}
	if err := s.checkDuplicates(userID, items); err != nil {
		return nil, err
	}
	res := &ParseResult{Items: items, TotalCount: len(items)}
	for _, it := range items {
		switch it.Type {
		case model.TypeExpense:
			res.Expenses++
		case model.TypeIncome:
			res.Incomes++
		}
		if it.IsDuplicate {
			res.Duplicates++
		}
		if it.IsFiltered {
			res.Filtered++
		}
	}
	return res, nil
}

// ==================== 解析 ====================

func (s *BillImportService) parseWechat(data []byte) ([]model.ImportItem, error) {
	// xlsx 本质是 zip。内部若含 .csv 直接按 CSV 解析，否则按标准 xlsx 解析。
	if isXLSX(data) {
		rc, err := findCSVInXLSX(data)
		if err == nil && rc != nil {
			defer rc.Close()
			raw, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			return s.parseWechatCSV(raw)
		}
		return s.parseWechatXLSX(data)
	}
	return s.parseWechatCSV(data)
}

func (s *BillImportService) parseWechatCSV(data []byte) ([]model.ImportItem, error) {
	rows, err := readCSV(data)
	if err != nil {
		return nil, fmt.Errorf("解析微信账单失败：%v", err)
	}
	// 找表头行（同时包含 交易时间 / 交易单号 / 金额 的列）
	headerIdx, start := findHeader(rows, []string{"time", "order_no", "amount"})
	if headerIdx == nil {
		return nil, errors.New("未能识别微信账单表头，请确认文件为微信支付导出的账单")
	}
	items := make([]model.ImportItem, 0, len(rows)-start)
	for _, row := range rows[start:] {
		if len(row) < len(headerIdx) || row[headerIdx["time"]] == "" {
			continue
		}
		it, ok := buildWechatItem(headerIdx, row)
		if !ok {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

func (s *BillImportService) parseWechatXLSX(data []byte) ([]model.ImportItem, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("无法读取微信账单 Excel：%v", err)
	}
	var sharedXML []byte
	var sheetXML []byte
	for _, f := range zr.File {
		switch f.Name {
		case "xl/sharedStrings.xml":
			sharedXML, err = readZipEntry(f)
		case "xl/worksheets/sheet1.xml":
			sheetXML, err = readZipEntry(f)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(sheetXML) == 0 {
		return nil, errors.New("微信账单 Excel 中没有可解析的工作表")
	}
	shared, err := parseXlsxSharedStrings(sharedXML)
	if err != nil {
		return nil, err
	}
	grid, err := parseXlsxSheet(sheetXML, shared)
	if err != nil {
		return nil, err
	}
	// 表头行
	headerIdx, start := findHeaderGrid(grid, []string{"time", "order_no", "amount"})
	if headerIdx == nil {
		return nil, errors.New("未能识别微信账单表头，请确认文件为微信支付导出的账单")
	}
	items := make([]model.ImportItem, 0, len(grid)-start)
	for _, row := range grid[start:] {
		if getCell(row, headerIdx, "time") == "" {
			continue
		}
		it, ok := buildWechatItemFromGrid(headerIdx, row)
		if !ok {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

func (s *BillImportService) parseAlipay(data []byte) ([]model.ImportItem, error) {
	rows, err := readCSV(data)
	if err != nil {
		return nil, fmt.Errorf("解析支付宝账单失败：%v", err)
	}
	headerIdx, start := findHeader(rows, []string{"time", "order_no", "amount"})
	if headerIdx == nil {
		return nil, errors.New("未能识别支付宝账单表头，请确认文件为支付宝导出的账单")
	}
	items := make([]model.ImportItem, 0, len(rows)-start)
	for _, row := range rows[start:] {
		if len(row) <= headerIdx["time"] || strings.TrimSpace(row[headerIdx["time"]]) == "" {
			continue
		}
		it, ok := buildAlipayItem(headerIdx, row)
		if !ok {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

// ==================== 通用解析工具 ====================

// readCSV 读取 CSV：自动处理 UTF-8 BOM / UTF-8 / GBK 编码。
func readCSV(data []byte) ([][]string, error) {
	text, err := decodeBytes(data)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	// 清理字段中的空白（订单号后可能带 \t）
	out := make([][]string, len(records))
	for i, rec := range records {
		out[i] = make([]string, len(rec))
		for j, v := range rec {
			out[i][j] = strings.TrimSpace(v)
		}
	}
	return out, nil
}

func decodeBytes(data []byte) (string, error) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:]), nil // UTF-8 BOM
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	out, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("文件编码无法识别：%v", err)
	}
	return string(out), nil
}

// findHeader 在 CSV 行中寻找包含指定关键列名的表头行，返回 列名→列号 映射与数据起始行号。
func findHeader(rows [][]string, required []string) (map[string]int, int) {
	for i, row := range rows {
		idx := map[string]int{}
		for j, cell := range row {
			c := strings.TrimSpace(cell)
			if c == "" {
				continue
			}
			switch c {
			case "交易时间":
				idx["time"] = j
			case "交易类型":
				idx["trade_type"] = j
			case "交易分类":
				idx["trade_type"] = j
			case "交易对方":
				idx["counterparty"] = j
			case "商品":
				idx["product"] = j
			case "商品说明":
				idx["product"] = j
			case "收/支":
				idx["flow"] = j
			case "金额(元)", "金额（元）", "金额":
				idx["amount"] = j
			case "支付方式":
				idx["pay_way"] = j
			case "收/付款方式":
				idx["pay_way"] = j
			case "当前状态":
				idx["status"] = j
			case "交易状态":
				idx["status"] = j
			case "交易单号":
				idx["order_no"] = j
			case "交易记录编号", "交易订单号":
				idx["order_no"] = j
			case "备注":
				idx["note"] = j
			}
		}
		// 必须包含所有必选列
		ok := true
		for _, r := range required {
			if _, found := idx[r]; !found {
				ok = false
				break
			}
		}
		if ok {
			return idx, i + 1
		}
	}
	return nil, 0
}

// buildWechatItem 由 CSV 行构建导入项。过滤项同样返回（带 is_filtered 标记，供展示）。
func buildWechatItem(h map[string]int, row []string) (model.ImportItem, bool) {
	raw := marshalRow(h, row)
	flow := cell(row, h, "flow")
	occurred, _ := parseTimeStr(cell(row, h, "time"))
	amount := parseAmount(cell(row, h, "amount"))
	status := cell(row, h, "status")
	product := cell(row, h, "product")
	tradeType := cell(row, h, "trade_type")
	counterparty := cell(row, h, "counterparty")
	note := buildNote(product, cell(row, h, "note"))

	base := model.ImportItem{
		PlatformOrderNo: cell(row, h, "order_no"),
		OccurredAt:      occurred,
		Amount:          amount,
		Counterparty:    counterparty,
		Note:            note,
		PayWay:          cell(row, h, "pay_way"),
		RawData:         raw,
	}
	switch flow {
	case "支出":
		base.Type = model.TypeExpense
	case "收入":
		base.Type = model.TypeIncome
	default:
		base.IsFiltered = true
		base.FilterReason = "不计入收支，未导入"
		return base, true
	}
	if reason := invalidWechatStatus(status); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	if reason := matchWealth(tradeType, product, counterparty); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	return base, true
}

// buildWechatItemFromGrid 由 xlsx 表格行构建导入项。
func buildWechatItemFromGrid(h map[string]int, row []string) (model.ImportItem, bool) {
	raw := marshalRow(h, row)
	flow := getCell(row, h, "flow")
	occurred := parseExcelTime(getCell(row, h, "time"))
	amount := parseAmount(getCell(row, h, "amount"))
	status := getCell(row, h, "status")
	product := getCell(row, h, "product")
	tradeType := getCell(row, h, "trade_type")
	counterparty := getCell(row, h, "counterparty")
	note := buildNote(product, getCell(row, h, "note"))

	base := model.ImportItem{
		PlatformOrderNo: getCell(row, h, "order_no"),
		OccurredAt:      occurred,
		Amount:          amount,
		Counterparty:    counterparty,
		Note:            note,
		PayWay:          getCell(row, h, "pay_way"),
		RawData:         raw,
	}
	switch flow {
	case "支出":
		base.Type = model.TypeExpense
	case "收入":
		base.Type = model.TypeIncome
	default:
		base.IsFiltered = true
		base.FilterReason = "不计入收支，未导入"
		return base, true
	}
	if reason := invalidWechatStatus(status); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	if reason := matchWealth(tradeType, product, counterparty); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	return base, true
}

// buildAlipayItem 由支付宝 CSV 行构建导入项。
func buildAlipayItem(h map[string]int, row []string) (model.ImportItem, bool) {
	raw := marshalRow(h, row)
	flow := cell(row, h, "flow")
	status := cell(row, h, "status")
	product := cell(row, h, "product")
	tradeType := cell(row, h, "trade_type")
	counterparty := cell(row, h, "counterparty")
	occurred, _ := parseTimeStr(cell(row, h, "time"))
	amount := parseAmount(cell(row, h, "amount"))

	base := model.ImportItem{
		PlatformOrderNo: cell(row, h, "order_no"),
		OccurredAt:      occurred,
		Amount:          amount,
		Counterparty:    counterparty,
		Note:            buildNote(product, cell(row, h, "note")),
		PayWay:          cell(row, h, "pay_way"),
		RawData:         raw,
	}
	switch flow {
	case "支出":
		base.Type = model.TypeExpense
	case "收入":
		base.Type = model.TypeIncome
	default:
		base.IsFiltered = true
		base.FilterReason = "不计收支，未导入"
		return base, true
	}
	if reason := invalidAlipayStatus(status); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	if reason := matchWealth(tradeType, product, counterparty); reason != "" {
		base.IsFiltered = true
		base.FilterReason = reason
		return base, true
	}
	return base, true
}

// invalidWechatStatus 判断微信"当前状态"是否应过滤，返回过滤原因（空串=有效）。
func invalidWechatStatus(status string) string {
	s := strings.TrimSpace(status)
	if s == "" || s == "/" {
		return ""
	}
	if strings.Contains(s, "退款") {
		return "交易已退款，未导入"
	}
	if strings.Contains(s, "失败") || strings.Contains(s, "关闭") || strings.Contains(s, "取消") || strings.Contains(s, "超时") {
		return "交易未成功，未导入"
	}
	return ""
}

// invalidAlipayStatus 判断支付宝"交易状态"是否应过滤。
func invalidAlipayStatus(status string) string {
	s := strings.TrimSpace(status)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "退款") {
		return "交易已退款，未导入"
	}
	if !strings.Contains(s, "成功") {
		return "交易未成功，未导入"
	}
	return ""
}

// matchWealth 检查是否命中理财/投资关键词。
func matchWealth(fields ...string) string {
	var buf strings.Builder
	for _, f := range fields {
		buf.WriteString(f)
		buf.WriteString(" ")
	}
	text := buf.String()
	for _, kw := range wealthKeywords {
		if strings.Contains(text, kw) {
			return "理财/投资类记录，未导入"
		}
	}
	return ""
}

// buildNote 组装备注：商品 + 原备注。
func buildNote(product, note string) string {
	var parts []string
	if p := strings.TrimSpace(product); p != "" && p != "/" {
		parts = append(parts, p)
	}
	if n := strings.TrimSpace(note); n != "" && n != "/" {
		parts = append(parts, n)
	}
	return strings.Join(parts, " / ")
}

// ==================== 去重检测 ====================

// checkDuplicates 标记重复：先与库中已有记录比对，再处理本批次内部重复。
func (s *BillImportService) checkDuplicates(userID uint, items []model.ImportItem) error {
	orderNos, err := s.existingOrderNos(userID)
	if err != nil {
		return err
	}
	fuzzyKeys, err := s.existingFuzzyKeys(userID)
	if err != nil {
		return err
	}
	batchOrder := map[string]bool{}
	batchFuzzy := map[string]bool{}
	for i := range items {
		it := &items[i]
		if it.IsFiltered {
			continue
		}
		no := it.PlatformOrderNo
		if no != "" && (orderNos[no] || batchOrder[no]) {
			it.IsDuplicate = true
			it.DuplicateWay = "order_no"
			continue
		}
		if no != "" {
			batchOrder[no] = true
		}
		key := s.makeFuzzyKey(it)
		if key != "" && (fuzzyKeys[key] || batchFuzzy[key]) {
			it.IsDuplicate = true
			it.DuplicateWay = "fuzzy"
			continue
		}
		if key != "" {
			batchFuzzy[key] = true
		}
	}
	return nil
}

func (s *BillImportService) existingOrderNos(userID uint) (map[string]bool, error) {
	var rows []struct {
		PlatformOrderNo string
	}
	err := s.db.Model(&model.BillImportItem{}).
		Select("bill_import_items.platform_order_no").
		Joins("JOIN bill_imports ON bill_imports.id = bill_import_items.import_id").
		Where("bill_imports.user_id = ? AND bill_import_items.platform_order_no != '' AND bill_import_items.status = ?",
			userID, model.ImportItemImported).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.PlatformOrderNo] = true
	}
	return out, nil
}

func (s *BillImportService) existingFuzzyKeys(userID uint) (map[string]bool, error) {
	var rows []struct {
		OccurredAt   time.Time
		Amount       float64
		Counterparty string
	}
	err := s.db.Model(&model.BillImportItem{}).
		Select("bill_import_items.occurred_at, bill_import_items.amount, bill_import_items.counterparty").
		Joins("JOIN bill_imports ON bill_imports.id = bill_import_items.import_id").
		Where("bill_imports.user_id = ? AND bill_import_items.counterparty != '' AND bill_import_items.status = ?",
			userID, model.ImportItemImported).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[makeFuzzyKey(r.OccurredAt, r.Amount, r.Counterparty)] = true
	}
	return out, nil
}

// makeFuzzyKey 模糊去重键：日期 + 金额取整 + 对方前4字符。
func makeFuzzyKey(t time.Time, amount float64, counterparty string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02") + "|" + strconv.Itoa(int(math.Floor(amount))) + "|" + headRunes(counterparty, 4)
}

func (s *BillImportService) makeFuzzyKey(it *model.ImportItem) string {
	t := parseItemTime(it.OccurredAt)
	if t.IsZero() {
		return ""
	}
	return makeFuzzyKey(t, it.Amount, it.Counterparty)
}

func headRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

// ==================== 确认导入 ====================

// ConfirmRequest 确认导入请求。
type ConfirmRequest struct {
	UserID    uint
	Platform  string
	FileName  string
	AccountID uint
	Items     []model.ImportItem
}

// ConfirmResult 确认结果。
type ConfirmResult struct {
	ImportID       uint   `json:"import_id"`
	ImportedCount  int    `json:"imported_count"`
	SkippedCount   int    `json:"skipped_count"`
	DuplicatedSkip int    `json:"duplicated_skip"`
	Message        string `json:"message"`
}

// Confirm 事务写入：创建导入任务 + 逐条生成账单与导入明细。
func (s *BillImportService) Confirm(req ConfirmRequest) (*ConfirmResult, error) {
	if req.AccountID == 0 {
		return nil, errors.New("请选择账户")
	}
	var acc model.Account
	if err := s.db.Where("id = ? AND user_id = ?", req.AccountID, req.UserID).First(&acc).Error; err != nil {
		return nil, errors.New("账户不存在")
	}
	imp := &model.BillImport{
		UserID:     req.UserID,
		Platform:   req.Platform,
		FileName:   req.FileName,
		TotalCount: len(req.Items),
		Status:     model.ImportStatusPending,
	}
	res := &ConfirmResult{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(imp).Error; err != nil {
			return err
		}
		for i := range req.Items {
			it := &req.Items[i]
			if !it.Selected || it.IsFiltered {
				if err := s.saveSkippedItem(tx, imp.ID, it, skipReason(it)); err != nil {
					return err
				}
				res.SkippedCount++
				continue
			}
			// 最后防线：订单号在库中已存在则跳过（防止预览后重复导入）
			if it.PlatformOrderNo != "" {
				exists, err := s.orderNoExists(tx, req.UserID, it.PlatformOrderNo)
				if err != nil {
					return err
				}
				if exists {
					if err := s.saveSkippedItem(tx, imp.ID, it, "重复导入：交易订单号已存在"); err != nil {
						return err
					}
					res.SkippedCount++
					res.DuplicatedSkip++
					continue
				}
			}
			catID, err := s.resolveCategory(tx, req.UserID, it)
			if err != nil {
				return fmt.Errorf("第 %d 条（%s）：%v", i+1, it.Counterparty, err)
			}
			accID := req.AccountID
			if it.AccountID != 0 {
				var cnt int64
				tx.Model(&model.Account{}).Where("id = ? AND user_id = ?", it.AccountID, req.UserID).Count(&cnt)
				if cnt == 0 {
					return fmt.Errorf("第 %d 条（%s）：账户不存在", i+1, it.Counterparty)
				}
				accID = it.AccountID
			}
			occurred := parseItemTime(it.OccurredAt)
			if occurred.IsZero() {
				occurred = time.Now()
			}
			bill := &model.Bill{
				UserID:     req.UserID,
				CategoryID: catID,
				AccountID:  &accID,
				Type:       it.Type,
				Amount:     model.Round2(it.Amount),
				Note:       it.Note,
				OccurredAt: model.DateTime{Time: occurred},
			}
			if err := tx.Create(bill).Error; err != nil {
				return err
			}
			item := &model.BillImportItem{
				ImportID:        imp.ID,
				BillID:          &bill.ID,
				PlatformOrderNo: it.PlatformOrderNo,
				OccurredAt:      occurred,
				Amount:          model.Round2(it.Amount),
				Type:            it.Type,
				Counterparty:    it.Counterparty,
				Note:            it.Note,
				Status:          model.ImportItemImported,
				RawData:         it.RawData,
			}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			res.ImportedCount++
		}
		imp.ImportedCount = res.ImportedCount
		imp.SkippedCount = res.SkippedCount
		imp.Status = model.ImportStatusCompleted
		if err := tx.Save(imp).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.ImportID = imp.ID
	res.Message = fmt.Sprintf("成功导入 %d 笔账单，跳过 %d 条", res.ImportedCount, res.SkippedCount)
	return res, nil
}

func (s *BillImportService) saveSkippedItem(tx *gorm.DB, importID uint, it *model.ImportItem, reason string) error {
	occurred := parseItemTime(it.OccurredAt)
	return tx.Create(&model.BillImportItem{
		ImportID:        importID,
		PlatformOrderNo: it.PlatformOrderNo,
		OccurredAt:      occurred,
		Amount:          it.Amount,
		Type:            it.Type,
		Counterparty:    it.Counterparty,
		Note:            it.Note,
		Status:          model.ImportItemSkipped,
		SkipReason:      reason,
		RawData:         it.RawData,
	}).Error
}

func skipReason(it *model.ImportItem) string {
	switch {
	case it.IsFiltered:
		return it.FilterReason
	case it.IsDuplicate:
		return "重复记录，未导入"
	default:
		return "未选中，未导入"
	}
}

// orderNoExists 检查订单号在该用户名下是否已存在。
func (s *BillImportService) orderNoExists(tx *gorm.DB, userID uint, orderNo string) (bool, error) {
	var cnt int64
	err := tx.Model(&model.BillImportItem{}).
		Joins("JOIN bill_imports ON bill_imports.id = bill_import_items.import_id").
		Where("bill_imports.user_id = ? AND bill_import_items.platform_order_no = ? AND bill_import_items.status = ?",
			userID, orderNo, model.ImportItemImported).
		Count(&cnt).Error
	return cnt > 0, err
}

// resolveCategory 解析分类：前端指定优先，否则按备注关键词智能匹配，兜底"其他"。
func (s *BillImportService) resolveCategory(tx *gorm.DB, userID uint, it *model.ImportItem) (uint, error) {
	if it.CategoryID != 0 {
		var cat model.Category
		if err := tx.Where("id = ? AND user_id = ? AND type = ?", it.CategoryID, userID, it.Type).First(&cat).Error; err == nil {
			return cat.ID, nil
		}
	}
	var cats []model.Category
	if err := tx.Where("user_id = ? AND type = ?", userID, it.Type).
		Order("sort_order asc, id asc").Find(&cats).Error; err != nil {
		return 0, err
	}
	if len(cats) == 0 {
		return 0, errors.New("没有可用的收支分类")
	}
	text := it.Note + " " + it.Counterparty
	for _, c := range cats {
		if c.Name != "其他" && strings.Contains(text, c.Name) {
			return c.ID, nil
		}
	}
	for _, c := range cats {
		if c.Name == "其他" {
			return c.ID, nil
		}
	}
	return cats[0].ID, nil
}

// ==================== 工具函数 ====================

func isXLSX(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte{0x50, 0x4B, 0x03, 0x04})
}

// findCSVInXLSX 在 xlsx（zip）中寻找第一个 .csv 条目，找不到返回 nil。
func findCSVInXLSX(data []byte) (io.ReadCloser, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			return f.Open()
		}
	}
	return nil, nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// parseXlsxSharedStrings 解析 sharedStrings.xml 为字符串切片。
func parseXlsxSharedStrings(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []string
	var cur strings.Builder
	inSI := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "si" {
				cur.Reset()
				inSI = true
			}
		case xml.CharData:
			if inSI {
				cur.Write(el)
			}
		case xml.EndElement:
			if el.Name.Local == "si" {
				out = append(out, cur.String())
				inSI = false
			}
		}
	}
	return out, nil
}

// parseXlsxSheet 解析 sheet XML 为按行列的字符串网格（列号按单元格引用展开）。
func parseXlsxSheet(data []byte, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var grid [][]string
	var curRow []string
	var curCell struct {
		ref string
		t   string
		val string
	}
	inRow := false
	inCell := false
	flushCell := func() {
		if !inCell {
			return
		}
		if curCell.ref == "" {
			// 无引用：追加到当前行末尾
			curRow = append(curRow, curCell.val)
		} else {
			colIdx := columnIndex(curCell.ref)
			for len(curRow) <= colIdx {
				curRow = append(curRow, "")
			}
			val := curCell.val
			if curCell.t == "s" {
				if n, err := strconv.Atoi(val); err == nil && n >= 0 && n < len(shared) {
					val = shared[n]
				}
			}
			curRow[colIdx] = val
		}
		curCell = struct {
			ref string
			t   string
			val string
		}{}
	}
	flushRow := func() {
		flushCell()
		if inRow {
			grid = append(grid, curRow)
			curRow = nil
			inRow = false
		}
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "row":
				flushRow()
				curRow = nil
				inRow = true
			case "c":
				flushCell()
				inCell = true
				curCell.ref = attr(el, "r")
				curCell.t = attr(el, "t")
			case "v":
				// 值累积到 curCell.val
			case "t":
				// 文本（inlineStr / rich text）同样累积到 val
			}
		case xml.CharData:
			if inCell {
				curCell.val += string(el)
			}
		case xml.EndElement:
			switch el.Name.Local {
			case "c":
				flushCell()
				inCell = false
			case "row":
				flushRow()
			}
		}
	}
	return grid, nil
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// columnIndex 将 "A19" 转为列号（A=0）。
func columnIndex(ref string) int {
	idx := 0
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			idx = idx*26 + int(ch-'A') + 1
		} else if ch >= 'a' && ch <= 'z' {
			idx = idx*26 + int(ch-'a') + 1
		} else {
			break
		}
	}
	if idx == 0 {
		return 0
	}
	return idx - 1
}

// findHeaderGrid 在 xlsx 网格中寻找表头行。
func findHeaderGrid(grid [][]string, required []string) (map[string]int, int) {
	for i, row := range grid {
		idx := map[string]int{}
		for j, cell := range row {
			switch strings.TrimSpace(cell) {
			case "交易时间":
				idx["time"] = j
			case "交易类型":
				idx["trade_type"] = j
			case "交易对方":
				idx["counterparty"] = j
			case "商品":
				idx["product"] = j
			case "收/支":
				idx["flow"] = j
			case "金额(元)", "金额（元）", "金额":
				idx["amount"] = j
			case "支付方式":
				idx["pay_way"] = j
			case "当前状态":
				idx["status"] = j
			case "交易单号":
				idx["order_no"] = j
			case "备注":
				idx["note"] = j
			}
		}
		ok := true
		for _, r := range required {
			if _, found := idx[r]; !found {
				ok = false
				break
			}
		}
		if ok {
			return idx, i + 1
		}
	}
	return nil, 0
}

func getCell(row []string, h map[string]int, key string) string {
	i, ok := h[key]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

func cell(row []string, h map[string]int, key string) string {
	return getCell(row, h, key)
}

// marshalRow 将原始行打包为 JSON 字符串（用于溯源）。
func marshalRow(h map[string]int, row []string) string {
	m := map[string]string{}
	for name, idx := range h {
		if idx >= 0 && idx < len(row) {
			m[name] = row[idx]
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "¥", "")
	s = strings.ReplaceAll(s, "￥", "")
	s = strings.ReplaceAll(s, "元", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Round(f*100) / 100
}

// parseItemTime 将导入项时间字符串解析为 time.Time（失败返回零值）。
func parseItemTime(s string) time.Time {
	if str, ok := parseTimeStr(s); ok {
		if t, err := time.ParseInLocation("2006-01-02 15:04", str, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseTimeStr(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/1/2 15:04:05",
		"2006/1/2 15:04",
		"2006/1/2",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t.Format("2006-01-02 15:04"), true
		}
	}
	return s, false
}

// parseExcelTime 将 Excel 日期序列号或文本时间转为 "2006-01-02 15:04"。
func parseExcelTime(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil && f > 20000 {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local)
		return base.Add(time.Duration(f * float64(24*time.Hour))).Format("2006-01-02 15:04")
	}
	if s, ok := parseTimeStr(v); ok {
		return s
	}
	return v
}
