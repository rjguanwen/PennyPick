<template>
  <div class="report-page">
    <div class="page-head">
      <span class="page-title">月度报告</span>
      <div class="gen-area">
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" :clearable="false" style="width: 140px" />
        <el-button type="primary" :loading="generating" @click="generate">生成{{ monthLabel }}报告</el-button>
      </div>
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

    <!-- 报告详情 -->
    <el-dialog v-model="detailVisible" :title="`${detailTitle} 月度报告`" width="780px" top="5vh">
      <template v-if="detail">
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
        <div v-if="detail.accounts.length" class="sec">
          <div class="sec-title">账户分布</div>
          <div v-for="a in detail.accounts" :key="a.account_id" class="acc-row2">
            <el-icon :size="16" class="acc-ic2"><component :is="a.icon || 'Wallet'" /></el-icon>
            <span class="acc-name2">{{ a.name }}</span>
            <span class="acc-exp2">支出 ¥{{ formatMoney(a.expense) }}</span>
            <span class="acc-inc2">收入 ¥{{ formatMoney(a.income) }}</span>
          </div>
        </div>

        <!-- 标签 -->
        <div v-if="detail.tags.length" class="sec">
          <div class="sec-title">支出标签</div>
          <div class="tag-wrap">
            <span v-for="t in detail.tags" :key="t.tag_id" class="tag-chip2">{{ t.name }} ¥{{ formatMoney(t.total) }}（{{ t.count }} 笔）</span>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { reportsApi } from '../api'
import { currentMonth, formatMoney } from '../utils/format'

const month = ref(currentMonth())
const reports = ref([])
const generating = ref(false)

const detailVisible = ref(false)
const detail = ref(null)

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

onMounted(load)
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
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 10px;
}
.exp {
  color: #f56c6c;
}
.inc {
  color: #67c23a;
}
/* 概览 */
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
.ov-label {
  font-size: 12px;
  color: #909399;
}
.ov-val {
  font-size: 17px;
  font-weight: 700;
  margin-top: 4px;
}
/* 区块 */
.sec {
  margin-bottom: 18px;
}
.sec-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
  border-left: 3px solid #409eff;
  padding-left: 8px;
}
/* 对比 */
.cmp-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 8px;
}
.cmp-label {
  width: 80px;
  font-weight: 600;
  color: #606266;
}
.cmp-item {
  font-size: 13px;
  color: #606266;
}
.cmp-item + .cmp-item {
  margin-left: 8px;
}
/* 分类 */
.cat-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
}
.cat-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.cat-name {
  width: 90px;
  font-size: 14px;
}
.cat-pct {
  width: 48px;
  font-size: 12px;
  color: #909399;
}
.cat-bar {
  flex: 1;
  height: 8px;
  background: #f2f3f5;
  border-radius: 4px;
  overflow: hidden;
}
.cat-bar-in {
  height: 100%;
  border-radius: 4px;
}
.cat-total {
  width: 90px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
}
.cat-chg {
  width: 56px;
  text-align: right;
  font-size: 12px;
}
/* 趋势 */
.trend-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f7fa;
  font-size: 13px;
}
.trend-month {
  width: 64px;
  color: #909399;
}
.trend-exp {
  flex: 1;
}
.trend-inc {
  flex: 1;
}
/* 账户 */
.acc-row2 {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f7fa;
  font-size: 13px;
}
.acc-ic2 {
  color: #409eff;
}
.acc-name2 {
  width: 120px;
}
.acc-exp2 {
  flex: 1;
  color: #f56c6c;
}
.acc-inc2 {
  flex: 1;
  color: #67c23a;
}
/* 标签 */
.tag-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.tag-chip2 {
  background: #ecf5ff;
  color: #409eff;
  border-radius: 8px;
  padding: 4px 10px;
  font-size: 13px;
}
.chg-good {
  color: #67c23a;
}
.chg-bad {
  color: #f56c6c;
}
.chg-na {
  color: #c0c4cc;
}
</style>
