// 金额格式化：1234.5 -> "1,234.50"
export function formatMoney(n) {
  const v = Number(n)
  if (isNaN(v)) return '0.00'
  return v.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

const pad = (n) => String(n).padStart(2, '0')

// Date -> "YYYY-MM-DD HH:mm"
export function fmtDateTime(d) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// Date -> "YYYY-MM-DD"
export function fmtDate(d) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function nowDateTime() {
  return fmtDateTime(new Date())
}

export function nowDate(d = new Date()) {
  return fmtDate(d)
}

// 当前月份 "YYYY-MM"
export function currentMonth() {
  const d = new Date()
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`
}

// 往前/后推 N 个月
export function shiftMonth(month, delta) {
  const [y, m] = month.split('-').map(Number)
  const d = new Date(y, m - 1 + delta, 1)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`
}

// 月份-1：得到上个月
export function prevMonth(month) {
  return shiftMonth(month, -1)
}

// "YYYY-MM" 中文展示："2026年8月"
export function monthLabel(month) {
  const [y, m] = month.split('-').map(Number)
  return `${y}年${m}月`
}

// "YYYY-MM-DD" -> 今天/昨天/8月19日
export function dateLabel(dateStr) {
  const today = new Date()
  const yesterday = new Date()
  yesterday.setDate(today.getDate() - 1)
  if (dateStr.slice(0, 10) === fmtDate(today)) return '今天'
  if (dateStr.slice(0, 10) === fmtDate(yesterday)) return '昨天'
  const [, m, d] = dateStr.slice(0, 10).split('-').map(Number)
  return `${m}月${d}日`
}

// 根据账单日期获取月份
export function monthOfDate(dateStr) {
  return dateStr.slice(0, 7)
}

// 金额输入相关：将字符串金额归一化
export function normalizeAmountInput(s) {
  let t = String(s).replace(/[^\d.]/g, '')
  // 只保留第一个小数点
  const idx = t.indexOf('.')
  if (idx !== -1) {
    t = t.slice(0, idx + 1) + t.slice(idx + 1).replace(/\./g, '')
    // 最多两位小数
    const [int, dec] = t.split('.')
    t = int.slice(0, 9) + '.' + dec.slice(0, 2)
  } else {
    t = t.slice(0, 9)
  }
  return t
}
