<template>
  <div class="report-page">
    <!-- 头部：标题 + 模式切换 -->
    <div class="page-head">
      <span class="page-title">收支报告</span>
      <el-radio-group v-model="mode">
        <el-radio-button value="month">月度报告</el-radio-button>
        <el-radio-button value="year">年度报告</el-radio-button>
      </el-radio-group>
    </div>

    <!-- ============ 月度模式 ============ -->
    <template v-if="mode === 'month'">
      <div class="gen-area">
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" :clearable="false" style="width: 140px" />
        <el-button type="primary" :loading="generating" @click="generate">生成{{ monthLabel }}报告</el-button>
      </div>

      <!-- 已生成报告列表 -->
      <div class="pp-card">
        <div class="card-title">已生成报告</div>
        <el-table :data="reports" size="small" style="width: 100%">
          <el-table-column prop="month" label="月份" width="100" />
          <el-table-column label="生成时间" width="170">
            <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="支出" width="130">
            <template #default="{ row }"><span class="exp">¥{{ formatMoney(row.expense_total) }}</span></template>
          </el-table-column>
          <el-table-column label="收入" width="130">
            <template #default="{ row }"><span class="inc">¥{{ formatMoney(row.income_total) }}</span></template>
          </el-table-column>
          <el-table-column label="结余">
            <template #default="{ row }"><span :class="row.income_total - row.expense_total >= 0 ? 'inc' : 'exp'">¥{{ formatMoney(row.income_total - row.expense_total) }}</span></template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ row }"><el-button link type="primary" size="small" @click="view(row)">查看</el-button></template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!reports.length" description="还没有生成过报告，选择月份后点击「生成」" :image-size="60" />
      </div>
    </template>

    <!-- ============ 年度模式 ============ -->
    <template v-else>
      <div class="gen-area">
        <el-date-picker v-model="year" type="year" value-format="YYYY" :clearable="false" style="width: 120px" />
        <el-button type="primary" :loading="generating" @click="generateYear">生成{{ year }}年度报告</el-button>
      </div>

      <!-- 已生成年度报告列表 -->
      <div class="pp-card">
        <div class="card-title">已生成年度报告</div>
        <el-table :data="yearReports" size="small" style="width: 100%">
          <el-table-column prop="year" label="年份" width="100" />
          <el-table-column label="生成时间" width="170">
            <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="支出" width="130">
            <template #default="{ row }"><span class="exp">¥{{ formatMoney(row.expense_total) }}</span></template>
          </el-table-column>
          <el-table-column label="收入" width="130">
            <template #default="{ row }"><span class="inc">¥{{ formatMoney(row.income_total) }}</span></template>
          </el-table-column>
          <el-table-column label="结余">
            <template #default="{ row }"><span :class="row.income_total - row.expense_total >= 0 ? 'inc' : 'exp'">¥{{ formatMoney(row.income_total - row.expense_total) }}</span></template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ row }"><el-button link type="primary" size="small" @click="viewYear(row)">查看</el-button></template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!yearReports.length" description="还没有生成过年度报告，选择年份后点击「生成」" :image-size="60" />
      </div>
    </template>

    <!-- ============ 月度报告弹窗 ============ -->
    <el-dialog v-model="detailVisible" :title="`${detailTitle} 月度报告`" width="820px" top="5vh" :close-on-click-modal="false">
      <div v-if="detail" ref="monthExportEl" data-report-export class="report-detail">
        <div class="report-toolbar">
          <span class="report-title">{{ detailTitle }} 月度报告</span>
          <el-button type="success" :icon="Download" :loading="exportingPdf" @click="exportMonthPdf">导出 PDF</el-button>
        </div>

        <!-- 概览 -->
        <div class="ov-grid">
          <div class="ov-card"><div class="ov-label">支出</div><div class="ov-val exp">¥{{ formatMoney(detail.overview.expense_total) }}</div></div>
          <div class="ov-card"><div class="ov-label">收入</div><div class="ov-val inc">¥{{ formatMoney(detail.overview.income_total) }}</div></div>
          <div class="ov-card"><div class="ov-label">结余</div><div class="ov-val">{{ formatMoney(detail.overview.balance) }}</div></div>
          <div class="ov-card"><div class="ov-label">笔数</div><div class="ov-val">{{ detail.overview.bill_count }}</div></div>
          <div class="ov-card"><div class="ov-label">日均支出</div><div class="ov-val exp">¥{{ formatMoney(detail.overview.daily_avg) }}</div></div>
        </div>

        <!-- 对比分析 -->
        <div class="sec">
          <div class="sec-title">对比分析</div>
          <div v-for="(cmp, label) in cmpList" :key="label" class="cmp-row">
            <span class="cmp-label">{{ label }}</span>
            <span class="cmp-item">支出 <b>¥{{ formatMoney(cmp.expense) }}</b> <b :class="pctClass(cmp.expense_change_pct, false)">{{ fmtPct(cmp.expense_change_pct) }}</b></span>
            <span class="cmp-item">收入 <b>¥{{ formatMoney(cmp.income) }}</b> <b :class="pctClass(cmp.income_change_pct, true)">{{ fmtPct(cmp.income_change_pct) }}</b></span>
          </div>
        </div>

        <!-- 分类 -->
        <div class="sec">
          <div class="sec-title">支出分类（与上月对比）</div>
          <div v-for="c in detail.categories" :key="c.category_id" class="cat-row">
            <span class="cat-dot" :style="{ background: c.color }"></span>
            <span class="cat-name">{{ c.name }}</span>
            <span class="cat-pct">{{ c.percent }}%</span>
            <div class="cat-bar"><div class="cat-bar-in" :style="{ width: Math.min(c.percent, 100) + '%', background: c.color }"></div></div>
            <span class="cat-total">¥{{ formatMoney(c.total) }}</span>
            <span class="cat-chg" :class="pctClass(c.change_pct, false)">{{ fmtPct(c.change_pct) }}</span>
          </div>
        </div>

        <!-- 趋势 -->
        <div class="sec">
          <div class="sec-title">近 6 个月收支趋势</div>
          <div v-for="t in detail.trend" :key="t.month" class="trend-row">
            <span class="trend-month">{{ fmtMonthShort(t.month) }}</span>
            <span class="trend-exp">支出 <b>¥{{ formatMoney(t.expense) }}</b></span>
            <span class="trend-inc">收入 <b>¥{{ formatMoney(t.income) }}</b></span>
          </div>
        </div>

        <!-- 账户 -->
        <div v-if="(detail.accounts || []).length" class="sec">
          <div class="sec-title">账户分布</div>
          <div v-for="a in detail.accounts || []" :key="a.account_id" class="acc-row2">
            <el-icon :size="16" class="acc-ic2"><component :is="a.icon || 'Wallet'" /></el-icon>
            <span class="acc-name2">{{ a.name }}</span>
            <span class="acc-exp2">支出 ¥{{ formatMoney(a.expense) }}</span>
            <span class="acc-inc2">收入 ¥{{ formatMoney(a.income) }}</span>
          </div>
        </div>

        <!-- 标签 -->
        <div v-if="(detail.tags || []).length" class="sec">
          <div class="sec-title">支出标签</div>
          <div class="tag-wrap">
            <span v-for="t in detail.tags || []" :key="t.tag_id" class="tag-chip2">{{ t.name }} ¥{{ formatMoney(t.total) }}（{{ t.count }} 笔）</span>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- ============ 年度报告弹窗 ============ -->
    <el-dialog v-model="yearVisible" :title="`${year} 年度收支报告`" width="920px" top="4vh" :close-on-click-modal="false" @opened="renderCharts">
      <div v-if="yearData" ref="yearExportEl" data-report-export class="report-detail">
        <div class="report-toolbar">
          <span class="report-title">{{ year }} 年度收支报告</span>
          <el-button type="success" :icon="Download" :loading="exportingPdf" @click="exportYearPdf">导出 PDF</el-button>
        </div>

        <!-- 概览 -->
        <div class="ov-grid">
          <div class="ov-card"><div class="ov-label">年支出</div><div class="ov-val exp">¥{{ formatMoney(yearData.expense_total) }}</div></div>
          <div class="ov-card"><div class="ov-label">年收入</div><div class="ov-val inc">¥{{ formatMoney(yearData.income_total) }}</div></div>
          <div class="ov-card"><div class="ov-label">结余</div><div class="ov-val">{{ formatMoney(yearData.balance) }}</div></div>
          <div class="ov-card"><div class="ov-label">笔数</div><div class="ov-val">{{ yearData.bill_count }}</div></div>
          <div class="ov-card"><div class="ov-label">退款合计</div><div class="ov-val inc">¥{{ formatMoney(yearData.refund_total) }}</div></div>
        </div>

        <!-- 全年月度收支趋势 -->
        <div class="sec">
          <div class="sec-title">全年月度收支趋势</div>
          <div ref="monthTrendChartEl" class="chart"></div>
        </div>

        <!-- 各账户按月收支趋势（所有账户一张图） -->
        <div class="sec">
          <div class="sec-title">各账户收支趋势</div>
          <div ref="accTrendChartEl" class="chart"></div>
          <p class="chart-note">数值为各账户当月净收支（收入 − 支出），负数表示该月净支出。</p>
        </div>

        <!-- 信用账户还款 -->
        <div v-if="(yearData.credit_repayment || []).length" class="sec">
          <div class="sec-title">信用账户还款情况</div>
          <el-table :data="repayRows" size="small" border style="width: 100%">
            <el-table-column prop="name" label="账户" width="110" fixed="left" />
            <el-table-column v-for="i in 12" :key="i" :label="i + '月'" align="center">
              <template #default="{ row }">
                <div v-if="row[i]" class="repay-cell">
                  <div class="rc-due">应还 ¥{{ formatMoney(row[i].due) }}</div>
                  <div class="rc-paid">
                    已还 ¥{{ formatMoney(row[i].amount) }}
                    <el-tag v-if="row[i].status === 'full'" size="small" type="success" effect="plain">还清</el-tag>
                    <el-tag v-else-if="row[i].status === 'partial'" size="small" type="warning" effect="plain">部分</el-tag>
                    <el-tag v-else size="small" type="danger" effect="plain">未还</el-tag>
                  </div>
                </div>
                <div v-else class="na">—</div>
              </template>
            </el-table-column>
            <el-table-column label="应还合计" align="center" width="110">
              <template #default="{ row }"><b class="due-total">¥{{ formatMoney(row.due_total) }}</b></template>
            </el-table-column>
            <el-table-column label="已还合计" align="center" width="110">
              <template #default="{ row }"><b>¥{{ formatMoney(row.total) }}</b></template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 支出分类 -->
        <div class="sec">
          <div class="sec-title">支出分类</div>
          <div v-for="c in yearData.categories || []" :key="c.category_id" class="cat-row">
            <span class="cat-dot" :style="{ background: c.color }"></span>
            <span class="cat-name">{{ c.name }}</span>
            <span class="cat-pct">{{ c.percent }}%</span>
            <div class="cat-bar"><div class="cat-bar-in" :style="{ width: Math.min(c.percent, 100) + '%', background: c.color }"></div></div>
            <span class="cat-total">¥{{ formatMoney(c.total) }}</span>
          </div>
          <el-empty v-if="!(yearData.categories || []).length" description="本年暂无支出" :image-size="50" />
        </div>

        <!-- 账户收支汇总 -->
        <div v-if="(yearData.account_summary || []).length" class="sec">
          <div class="sec-title">账户收支汇总</div>
          <div v-for="a in yearData.account_summary || []" :key="a.account_id" class="acc-row2">
            <el-icon :size="16" class="acc-ic2"><component :is="a.icon || 'Wallet'" /></el-icon>
            <span class="acc-name2">{{ a.name }}</span>
            <span class="acc-exp2">支出 ¥{{ formatMoney(a.expense) }}</span>
            <span class="acc-inc2">收入 ¥{{ formatMoney(a.income) }}</span>
          </div>
        </div>

        <!-- 支出标签 -->
        <div v-if="(yearData.tags || []).length" class="sec">
          <div class="sec-title">支出标签</div>
          <div class="tag-wrap">
            <span v-for="t in yearData.tags || []" :key="t.tag_id" class="tag-chip2">{{ t.name }} ¥{{ formatMoney(t.total) }}（{{ t.count }} 笔）</span>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import html2canvas from 'html2canvas'
import { jsPDF } from 'jspdf'
import { reportsApi } from '../api'
import { currentMonth, formatMoney } from '../utils/format'

const mode = ref('month')
const month = ref(currentMonth())
const reports = ref([])
const generating = ref(false)

const detailVisible = ref(false)
const detail = ref(null)

const year = ref(String(new Date().getFullYear()))
const yearData = ref(null)
const yearVisible = ref(false)
const yearReports = ref([])

const exportingPdf = ref(false)

// 图表实例
let monthTrendChart = null
let accTrendChart = null
const monthTrendChartEl = ref(null)
const accTrendChartEl = ref(null)
const monthExportEl = ref(null)
const yearExportEl = ref(null)

const monthLabel = computed(() => {
  const [y, m] = month.value.split('-')
  return `${y}年${Number(m)}月`
})
const detailTitle = computed(() => {
  const [y, m] = (detail.value?.month || '').split('-')
  return `${y}年${Number(m)}月`
})
const cmpList = computed(() => {
  if (!detail.value) return {}
  return {
    与上月: detail.value.overview.prev_month,
    与去年同期: detail.value.overview.last_year,
  }
})

// 信用还款表格行：每月同时展示应还金额与已还金额
const repayRows = computed(() => {
  const list = yearData.value?.credit_repayment || []
  return list.map((a) => {
    const row = { name: a.name, total: a.total, due_total: a.due_total || 0 }
    a.months.forEach((m) => {
      const mm = Number(m.month.slice(5, 7))
      if (Number(m.due) > 0 || Number(m.amount) > 0) {
        row[mm] = { due: m.due || 0, amount: m.amount || 0, status: m.status || '' }
      }
    })
    return row
  })
})

function fmtTime(t) {
  return t ? String(t).slice(0, 16).replace('T', ' ') : ''
}
function fmtMonthShort(m) {
  const [y, mm] = m.split('-')
  return `${y}.${mm}`
}
function fmtPct(pct) {
  if (pct == null) return '—'
  const v = Number(pct)
  return `${v > 0 ? '+' : ''}${v}%`
}
function pctClass(pct, isIncome) {
  if (pct == null) return 'chg-na'
  const v = Number(pct)
  if (v === 0) return 'chg-na'
  const bad = isIncome ? v < 0 : v > 0
  return bad ? 'chg-bad' : 'chg-good'
}

// ---------- 月度报告 ----------
async function load() {
  reports.value = (await reportsApi.list()) || []
}
async function generate() {
  generating.value = true
  try {
    const res = await reportsApi.generate({ month: month.value })
    ElMessage.success(`${monthLabel.value}报告已生成${res.created_at ? '（覆盖旧报告）' : ''}`)
    load()
    detail.value = res.data
    detailVisible.value = true
  } catch (e) {
    // 拦截器已提示
  } finally {
    generating.value = false
  }
}
async function view(row) {
  try {
    const res = await reportsApi.detail(row.id)
    detail.value = res.data
    detailVisible.value = true
  } catch (e) {
    // 拦截器已提示
  }
}

// ---------- 年度报告 ----------
async function loadYearReports() {
  yearReports.value = (await reportsApi.yearlyList()) || []
}
async function generateYear() {
  if (!year.value) return
  generating.value = true
  try {
    const res = await reportsApi.yearlyGenerate({ year: year.value })
    ElMessage.success(`${year.value}年度报告已生成${res.created_at ? '（重新生成覆盖旧报告）' : ''}`)
    loadYearReports()
    yearData.value = res.data
    yearVisible.value = true
  } catch (e) {
    // 拦截器已提示
  } finally {
    generating.value = false
  }
}
async function viewYear(row) {
  try {
    const res = await reportsApi.yearlyDetail(row.id)
    yearData.value = res.data
    yearVisible.value = true
  } catch (e) {
    // 拦截器已提示
  }
}

// ---------- 图表（年度弹窗打开后渲染）----------
function chartBase() {
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
  }
}
function monthLabels() {
  const arr = []
  for (let i = 1; i <= 12; i++) arr.push(`${i}月`)
  return arr
}

function renderCharts() {
  if (!yearData.value || !yearVisible.value) return
  const labels = monthLabels()

  // 全年月度收支趋势
  if (monthTrendChartEl.value) {
    monthTrendChart?.dispose()
    monthTrendChart = echarts.init(monthTrendChartEl.value)
    monthTrendChart.setOption({
      ...chartBase(),
      legend: { data: ['支出', '收入'] },
      xAxis: { type: 'category', data: labels },
      yAxis: { type: 'value', name: '金额(¥)' },
      series: [
        { name: '支出', type: 'line', smooth: true, data: yearData.value.monthly_trend.map((m) => m.expense), itemStyle: { color: '#f56c6c' } },
        { name: '收入', type: 'line', smooth: true, data: yearData.value.monthly_trend.map((m) => m.income), itemStyle: { color: '#67c23a' } },
      ],
    })
  }

  // 各账户收支趋势（一张图，净收支）
  if (accTrendChartEl.value) {
    accTrendChart?.dispose()
    accTrendChart = echarts.init(accTrendChartEl.value)
    const accList = yearData.value.account_trend || []
    const names = accList.map((a) => a.name)
    const series = accList.map((a) => ({
      name: a.name,
      type: 'line',
      smooth: true,
      data: a.months.map((m) => Math.round((m.income - m.expense) * 100) / 100),
    }))
    accTrendChart.setOption({
      ...chartBase(),
      // 底部图例与 x 轴标签分离：底部预留图例空间，避免重叠
      grid: { left: 50, right: 20, top: 40, bottom: 70 },
      legend: { data: names, type: 'scroll', bottom: 0 },
      tooltip: {
        trigger: 'axis',
        valueFormatter: (v) => `¥${formatMoney(v)}`,
      },
      xAxis: { type: 'category', data: labels },
      yAxis: { type: 'value', name: '净收支(¥)' },
      series,
    })
  }
}

// ---------- PDF 导出 ----------
async function exportElToPDF(el, filename) {
  if (!el) return
  exportingPdf.value = true
  try {
    await new Promise((r) => setTimeout(r, 200)) // 等待图表渲染稳定
    const canvas = await html2canvas(el, {
      scale: 2,
      useCORS: true,
      backgroundColor: '#ffffff',
      logging: false,
      onclone: (doc) => {
        // 解除弹窗滚动容器的裁剪，让报告内容完整展开（供 PDF 截图）
        const node = doc.querySelector('[data-report-export]')
        if (node) {
          let p = node
          while (p && p !== doc.body) {
            if (p.style) {
              p.style.overflow = 'visible'
              p.style.maxHeight = 'none'
              p.style.height = 'auto'
            }
            p = p.parentElement
          }
        }
      },
    })
    const pdf = new jsPDF('p', 'mm', 'a4')
    const M = 10 // 页边距 mm（避免内容贴边）
    const pw = pdf.internal.pageSize.getWidth() // 210
    const ph = pdf.internal.pageSize.getHeight() // 297
    const imgW = pw - M * 2 // 内容宽度
    const pageCanvasH = ((ph - M * 2) * canvas.width) / imgW // 每页可容纳的 canvas 像素高度

    // 各区块顶部在 canvas 中的坐标，用于对齐分页（避免切断标题/区块）
    const rectEl = el.getBoundingClientRect()
    const scaleRatio = canvas.width / rectEl.width
    const bounds = []
    el.querySelectorAll('.report-toolbar, .ov-grid, .sec').forEach((s) => {
      const r = s.getBoundingClientRect()
      bounds.push(Math.round((r.top - rectEl.top) * scaleRatio))
    })
    bounds.sort((a, b) => a - b)

    let srcY = 0
    let pageNo = 0
    while (srcY < canvas.height) {
      let pageEnd = srcY + pageCanvasH
      if (pageNo > 0) {
        // 页尾附近（约 20mm）若有区块边界，提前在该边界分页，保证区块完整
        const cutoff = Math.round((20 * canvas.width) / imgW)
        const nextBound = bounds.find((b) => b >= pageEnd - cutoff && b > srcY + 1)
        if (nextBound && nextBound < pageEnd) {
          pageEnd = nextBound
        }
      }
      const shown = Math.min(pageEnd, canvas.height) - srcY
      if (shown <= 0) break
      if (pageNo > 0) pdf.addPage()
      const shownH = (shown * imgW) / canvas.width
      // 裁剪当前页对应的 canvas 区域，输出本页（带边距）
      const pageCanvas = document.createElement('canvas')
      pageCanvas.width = canvas.width
      pageCanvas.height = Math.round(shown)
      pageCanvas.getContext('2d').drawImage(canvas, 0, Math.round(srcY), canvas.width, Math.round(shown), 0, 0, canvas.width, Math.round(shown))
      const pageData = pageCanvas.toDataURL('image/jpeg', 0.92)
      pdf.addImage(pageData, 'JPEG', M, M, imgW, shownH)
      srcY = pageEnd
      pageNo++
    }
    pdf.save(filename)
  } finally {
    exportingPdf.value = false
  }
}
function exportMonthPdf() {
  exportElToPDF(monthExportEl.value, `${detail.value?.month || ''}_月度报告.pdf`)
}
function exportYearPdf() {
  exportElToPDF(yearExportEl.value, `${year.value}_年度收支报告.pdf`)
}

onMounted(() => {
  load()
  loadYearReports()
})
onBeforeUnmount(() => {
  monthTrendChart?.dispose()
  accTrendChart?.dispose()
})
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.page-title {
  font-size: 17px;
  font-weight: 700;
}
.gen-area {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 10px;
}
.report-detail {
  padding-right: 4px;
}
.report-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.report-title {
  font-size: 16px;
  font-weight: 700;
  color: #303133;
}
.chart {
  width: 100%;
  height: 320px;
}
.chart-note {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 6px;
}
.exp { color: #f56c6c; }
.inc { color: #67c23a; }
.na { color: #c0c4cc; }
.repay-cell { line-height: 1.6; }
.rc-due { font-size: 12px; color: #909399; }
.rc-paid { font-size: 13px; color: #303133; }
.due-total { color: #e6a23c; }
.ov-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}
.ov-card {
  background: #f5f7fa;
  border-radius: 10px;
  padding: 12px 8px;
  text-align: center;
}
.ov-label { font-size: 12px; color: #909399; }
.ov-val { font-size: 17px; font-weight: 700; margin-top: 4px; }
.sec { margin-bottom: 18px; }
.sec-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
  border-left: 3px solid #409eff;
  padding-left: 8px;
}
.cmp-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 8px;
}
.cmp-label { width: 80px; font-weight: 600; color: #606266; }
.cmp-item { font-size: 13px; color: #606266; }
.cmp-item + .cmp-item { margin-left: 8px; }
.cat-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
}
.cat-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.cat-name { width: 90px; font-size: 14px; }
.cat-pct { width: 48px; font-size: 12px; color: #909399; }
.cat-bar { flex: 1; height: 8px; background: #f2f3f5; border-radius: 4px; overflow: hidden; }
.cat-bar-in { height: 100%; border-radius: 4px; }
.cat-total { width: 90px; text-align: right; font-size: 13px; font-weight: 600; }
.cat-chg { width: 56px; text-align: right; font-size: 12px; }
.trend-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f7fa;
  font-size: 13px;
}
.trend-month { width: 64px; color: #909399; }
.trend-exp { flex: 1; }
.trend-inc { flex: 1; }
.acc-row2 {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f7fa;
  font-size: 13px;
}
.acc-ic2 { color: #409eff; }
.acc-name2 { width: 120px; }
.acc-exp2 { flex: 1; color: #f56c6c; }
.acc-inc2 { flex: 1; color: #67c23a; }
.tag-wrap { display: flex; flex-wrap: wrap; gap: 8px; }
.tag-chip2 {
  background: #ecf5ff;
  color: #409eff;
  border-radius: 8px;
  padding: 4px 10px;
  font-size: 13px;
}
.chg-good { color: #67c23a; }
.chg-bad { color: #f56c6c; }
.chg-na { color: #c0c4cc; }
</style>
