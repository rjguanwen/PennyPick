<template>
  <div class="stats">
    <!-- 月份切换 -->
    <div class="head">
      <div class="month-switch">
        <button type="button" class="pp-tap" @click="changeMonth(-1)"><el-icon><ArrowLeft /></el-icon></button>
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" class="month-picker" @change="loadAll" />
        <button type="button" class="pp-tap" @click="changeMonth(1)"><el-icon><ArrowRight /></el-icon></button>
      </div>
    </div>

    <!-- 汇总卡片 -->
    <div class="stat-grid">
      <div class="pp-card stat-card">
        <div class="label">支出</div>
        <div class="value money-expense">¥{{ formatMoney(overview.expense_total) }}</div>
      </div>
      <div class="pp-card stat-card">
        <div class="label">收入</div>
        <div class="value money-income">¥{{ formatMoney(overview.income_total) }}</div>
      </div>
      <div class="pp-card stat-card">
        <div class="label">结余</div>
        <div class="value" :class="overview.balance >= 0 ? 'money-income' : 'money-expense'">¥{{ formatMoney(overview.balance) }}</div>
      </div>
      <div class="pp-card stat-card">
        <div class="label">日均支出</div>
        <div class="value">¥{{ formatMoney(dailyAvg) }}</div>
      </div>
    </div>

    <!-- 分类占比 -->
    <div class="pp-card chart-card">
      <div class="chart-head">
        <span class="chart-title">分类占比</span>
        <el-radio-group v-model="catType" size="small" @change="loadCategory">
          <el-radio-button value="expense">支出</el-radio-button>
          <el-radio-button value="income">收入</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="catData.length" ref="pieRef" class="chart pie"></div>
      <el-empty v-else description="本月暂无数据" :image-size="60" />
      <div v-if="catData.length" class="cat-list">
        <div v-for="c in catData" :key="c.category_id" class="cat-row">
          <span class="dot" :style="{ background: c.color }"></span>
          <span class="cat-name">{{ c.name }}</span>
          <span class="cat-percent">{{ c.percent.toFixed(1) }}%</span>
          <span class="cat-total money-expense">¥{{ formatMoney(c.total) }}</span>
        </div>
      </div>
    </div>

    <!-- 趋势 -->
    <div class="pp-card chart-card">
      <div class="chart-head">
        <span class="chart-title">收支趋势</span>
        <el-radio-group v-model="trendGranularity" size="small" @change="loadTrend">
          <el-radio-button value="month">按月</el-radio-button>
          <el-radio-button value="day">按日</el-radio-button>
        </el-radio-group>
      </div>
      <div ref="trendRef" class="chart trend"></div>
    </div>

    <!-- 标签统计 -->
    <div class="pp-card">
      <div class="chart-head">
        <span class="chart-title">标签统计</span>
        <el-radio-group v-model="tagType" size="small" @change="loadTags">
          <el-radio-button value="expense">支出</el-radio-button>
          <el-radio-button value="income">收入</el-radio-button>
        </el-radio-group>
      </div>
      <el-empty v-if="!tagsData.length" description="本月暂无标签数据" :image-size="50" />
      <div v-for="t in tagsData" :key="t.tag_id" class="tag-row">
        <el-tag size="small" type="info" effect="plain" class="tag-chip">{{ t.name }}</el-tag>
        <span class="tag-count">{{ t.bill_count }} 笔</span>
        <span class="tag-percent">{{ t.percent.toFixed(1) }}%</span>
        <span class="tag-total" :class="tagType === 'income' ? 'money-income' : 'money-expense'">¥{{ formatMoney(t.total) }}</span>
      </div>
    </div>

    <!-- 账户统计 -->
    <div class="pp-card">
      <div class="chart-head">
        <span class="chart-title">账户收支</span>
      </div>
      <el-empty v-if="!accountsData.length" description="本月暂无账户流水" :image-size="50" />
      <div v-for="a in accountsData" :key="a.account_id" class="acc-row">
        <CatIcon :icon="a.icon" color="#409eff" :size="16" />
        <span class="acc-name">{{ a.name }}</span>
        <span class="acc-income">收 ¥{{ formatMoney(a.income) }}</span>
        <span class="acc-expense">支 ¥{{ formatMoney(a.expense) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { accountApi, categoryApi, statsApi } from '../api'
import { currentMonth, formatMoney, shiftMonth } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'

const month = ref(currentMonth())
const overview = ref({ expense_total: 0, income_total: 0, balance: 0 })
const catType = ref('expense')
const catData = ref([])
const tagType = ref('expense')
const tagsData = ref([])
const trendGranularity = ref('month')
const trendData = ref([])
const accountsData = ref([])

const pieRef = ref(null)
const trendRef = ref(null)
let pieChart = null
let trendChart = null

const dailyAvg = computed(() => {
  const days = new Date(Number(month.value.slice(0, 4)), Number(month.value.slice(5)), 0).getDate()
  return days ? overview.value.expense_total / days : 0
})

function changeMonth(delta) {
  month.value = shiftMonth(month.value, delta)
  loadAll()
}

async function loadAll() {
  await Promise.all([loadOverview(), loadCategory(), loadTags(), loadTrend(), loadAccounts()])
}

async function loadOverview() {
  try {
    overview.value = await statsApi.overview(month.value)
  } catch (e) {}
}

async function loadCategory() {
  try {
    catData.value = (await statsApi.byCategory({ month: month.value, type: catType.value })) || []
    nextTick(renderPie)
  } catch (e) {}
}

async function loadTrend() {
  let params
  if (trendGranularity.value === 'day') {
    const y = Number(month.value.slice(0, 4))
    const m = Number(month.value.slice(5))
    const lastDay = new Date(y, m, 0).getDate()
    params = { granularity: 'day', start: `${month.value}-01`, end: `${month.value}-${String(lastDay).padStart(2, '0')}` }
  } else {
    const end = month.value
    const y = Number(end.slice(0, 4))
    const m = Number(end.slice(5))
    const startY = m >= 12 ? y : y - 1
    const startM = m >= 12 ? '12' : String(m + 1).padStart(2, '0')
    params = { granularity: 'month', start: `${startY}-${startM}`, end }
  }
  try {
    trendData.value = (await statsApi.trend(params)) || []
    nextTick(renderTrend)
  } catch (e) {}
}

async function loadAccounts() {
  try {
    accountsData.value = (await statsApi.accounts(month.value)) || []
  } catch (e) {}
}

async function loadTags() {
  try {
    tagsData.value = (await statsApi.tags({ month: month.value, type: tagType.value })) || []
  } catch (e) {}
}

function renderPie() {
  if (!pieRef.value) return
  if (!pieChart) pieChart = echarts.init(pieRef.value)
  const colors = catData.value.map((c) => c.color)
  pieChart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: (p) => `${p.name}<br/>¥${formatMoney(p.value)}（${p.percent}%）`,
    },
    legend: { show: false },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        center: ['50%', '50%'],
        itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
        label: { show: false },
        data: catData.value.map((c) => ({ name: c.name, value: Math.round(c.total * 100) / 100, itemStyle: { color: c.color } })),
      },
    ],
    color: colors,
  })
}

function renderTrend() {
  if (!trendRef.value) return
  if (!trendChart) trendChart = echarts.init(trendRef.value)
  const labels = trendData.value.map((t) => t.label)
  trendChart.setOption({
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v) => '¥' + formatMoney(v),
    },
    legend: { data: ['支出', '收入'], top: 0 },
    grid: { left: 10, right: 10, top: 34, bottom: 8, containLabel: true },
    xAxis: { type: 'category', data: labels, axisLabel: { fontSize: 11 } },
    yAxis: {
      type: 'value',
      axisLabel: { formatter: (v) => (v >= 10000 ? v / 10000 + 'w' : v) },
      splitLine: { lineStyle: { color: '#f0f0f0' } },
    },
    series: [
      { name: '支出', type: 'bar', data: trendData.value.map((t) => t.expense), itemStyle: { color: '#f56c6c', borderRadius: [4, 4, 0, 0] } },
      { name: '收入', type: 'line', smooth: true, data: trendData.value.map((t) => t.income), itemStyle: { color: '#67c23a' } },
    ],
  })
}

function onResize() {
  pieChart?.resize()
  trendChart?.resize()
}

onMounted(async () => {
  const [cats] = await Promise.all([categoryApi.list()])
  // 预取分类数据（供未来扩展）
  void cats
  await loadAll()
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  pieChart?.dispose()
  trendChart?.dispose()
})
</script>

<style scoped>
.stats {
  max-width: 1000px;
  margin: 0 auto;
}
.head {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
.month-switch {
  display: flex;
  align-items: center;
  gap: 6px;
}
.month-switch button {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid #dcdfe6;
  background: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #606266;
}
.month-picker {
  width: 130px;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}
.stat-card .label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 8px;
}
.stat-card .value {
  font-size: 19px;
  font-weight: 700;
  word-break: break-all;
}
.chart-card {
  margin-bottom: 12px;
}
.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.chart-title {
  font-weight: 600;
}
.chart {
  width: 100%;
}
.pie {
  height: 260px;
}
.trend {
  height: 300px;
}
.cat-list {
  margin-top: 10px;
}
.cat-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 14px;
}
.cat-row .dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.cat-name {
  flex: 1;
  color: #606266;
}
.cat-percent {
  color: #909399;
  font-size: 12px;
  width: 56px;
  text-align: right;
}
.cat-total {
  width: 110px;
  text-align: right;
}
.tag-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 0;
  border-bottom: 1px solid #f2f3f5;
  font-size: 14px;
}
.tag-row:last-child {
  border-bottom: none;
}
.tag-chip {
  flex-shrink: 0;
}
.tag-count {
  color: #909399;
  font-size: 12px;
  width: 52px;
}
.tag-percent {
  color: #909399;
  font-size: 12px;
  width: 56px;
  text-align: right;
}
.tag-total {
  flex: 1;
  text-align: right;
  font-weight: 600;
}
.acc-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid #f2f3f5;
}
.acc-row:last-child {
  border-bottom: none;
}
.acc-name {
  flex: 1;
  font-size: 15px;
}
.acc-income {
  color: #67c23a;
  font-size: 13px;
  width: 100px;
  text-align: right;
}
.acc-expense {
  color: #f56c6c;
  font-size: 13px;
  width: 100px;
  text-align: right;
}
</style>
