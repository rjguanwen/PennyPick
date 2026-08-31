<template>
  <div class="aq-page">
    <div class="page-head">
      <span class="page-title">账户查询</span>
      <span class="page-sub">查询各账户每月支出：信用账户按账期统计，非信用账户按自然月统计</span>
    </div>

    <!-- 查询条件 -->
    <div class="query-bar">
      <span class="q-label">开始月份</span>
      <el-date-picker v-model="start" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" style="width: 140px" />
      <span class="q-label">结束月份</span>
      <el-date-picker v-model="end" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" style="width: 140px" />
      <el-button type="primary" :icon="Search" :loading="loading" @click="load">查询</el-button>
    </div>

    <!-- 信用账户 -->
    <div v-if="creditRows.length" class="pp-card">
      <div class="card-title"><el-icon><CreditCard /></el-icon> 信用账户（按账期统计）</div>
      <el-table :data="creditRows" size="small" border style="width: 100%" v-loading="loading">
        <el-table-column label="账户" width="130" fixed="left">
          <template #default="{ row }">
            <el-icon :size="15" class="acc-ic"><component :is="row.icon || 'Wallet'" /></el-icon>
            <span class="acc-name">{{ row.name }}</span>
            <span v-if="row.statement_day" class="acc-day">出账{{ row.statement_day }}日</span>
          </template>
        </el-table-column>
        <el-table-column v-for="(m, i) in months" :key="m" :label="monthLabel(m)" align="right" :width="cellW">
          <template #default="{ row }">
            <button v-if="row.months[i].expense > 0" type="button" class="cell-btn" @click="openDetail(row, row.months[i])">
              ¥{{ formatMoney(row.months[i].expense) }}
            </button>
            <span v-else class="cell-zero">—</span>
          </template>
        </el-table-column>
        <el-table-column label="合计" align="right" width="110" fixed="right">
          <template #default="{ row }"><b class="cell-total">¥{{ formatMoney(rowTotal(row)) }}</b></template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 非信用账户 -->
    <div v-if="normalRows.length" class="pp-card">
      <div class="card-title"><el-icon><Wallet /></el-icon> 非信用账户（按自然月统计）</div>
      <el-table :data="normalRows" size="small" border style="width: 100%" v-loading="loading">
        <el-table-column label="账户" width="130" fixed="left">
          <template #default="{ row }">
            <el-icon :size="15" class="acc-ic"><component :is="row.icon || 'Wallet'" /></el-icon>
            <span class="acc-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column v-for="(m, i) in months" :key="m" :label="monthLabel(m)" align="right" :width="cellW">
          <template #default="{ row }">
            <button v-if="row.months[i].expense > 0" type="button" class="cell-btn" @click="openDetail(row, row.months[i])">
              ¥{{ formatMoney(row.months[i].expense) }}
            </button>
            <span v-else class="cell-zero">—</span>
          </template>
        </el-table-column>
        <el-table-column label="合计" align="right" width="110" fixed="right">
          <template #default="{ row }"><b class="cell-total">¥{{ formatMoney(rowTotal(row)) }}</b></template>
        </el-table-column>
      </el-table>
    </div>

    <el-empty v-if="!loading && !creditRows.length && !normalRows.length" description="当前范围内没有账户数据" :image-size="60" />

    <!-- 明细弹窗 -->
    <el-dialog v-model="detailVisible" :title="detailTitle" width="720px" :close-on-click-modal="false">
      <el-table :data="detailItems" size="small" style="width: 100%">
        <el-table-column prop="occurred_at" label="时间" width="150" />
        <el-table-column label="分类" width="110">
          <template #default="{ row }">
            <el-icon :size="14" class="acc-ic"><component :is="row.category_icon || 'Food'" /></el-icon>
            {{ row.category_name || '未分类' }}
          </template>
        </el-table-column>
        <el-table-column label="金额" width="120" align="right">
          <template #default="{ row }"><span class="exp">¥{{ formatMoney(row.amount) }}</span></template>
        </el-table-column>
        <el-table-column prop="note" label="备注" show-overflow-tooltip />
      </el-table>
      <el-empty v-if="!detailItems.length" description="该期间没有支出明细" :image-size="50" />
      <template #footer>
        <span class="detail-total">合计：<b>¥{{ formatMoney(detailTotal) }}</b></span>
        <el-button type="primary" @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { accountQueryApi } from '../api'
import { formatMoney } from '../utils/format'

const now = new Date()
const start = ref(`${now.getFullYear()}-01`)
const end = ref(`${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`)

const loading = ref(false)
const months = ref([])
const creditRows = ref([])
const normalRows = ref([])

const detailVisible = ref(false)
const detailTitle = ref('')
const detailItems = ref([])

const cellW = computed(() => {
  const n = months.value.length || 1
  return Math.max(90, Math.min(130, Math.floor(760 / n)))
})

const detailTotal = computed(() => detailItems.value.reduce((s, x) => s + Number(x.amount || 0), 0))

function monthLabel(m) {
  return `${Number(m.slice(5, 7))}月`
}

function rowTotal(row) {
  return (row.months || []).reduce((s, m) => s + Number(m.expense || 0), 0)
}

async function load() {
  if (!start.value || !end.value) {
    return
  }
  if (start.value > end.value) {
    ElMessage.warning('结束月份不能早于开始月份')
    return
  }
  // 跨度校验（最多 12 个月）
  const [sy, sm] = start.value.split('-').map(Number)
  const [ey, em] = end.value.split('-').map(Number)
  const span = (ey - sy) * 12 + (em - sm) + 1
  if (span > 12) {
    ElMessage.warning('查询月份跨度不能超过 12 个月')
    return
  }
  loading.value = true
  try {
    const res = await accountQueryApi.query({ start: start.value, end: end.value })
    months.value = res.months || []
    creditRows.value = res.credit_accounts || []
    normalRows.value = res.normal_accounts || []
  } catch (e) {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}

async function openDetail(row, item) {
  detailTitle.value = `${row.name} · ${item.month.slice(0, 4)}年${Number(item.month.slice(5, 7))}月支出明细`
  detailItems.value = []
  detailVisible.value = true
  try {
    const res = await accountQueryApi.bills({ account_id: row.account_id, month: item.month })
    detailItems.value = res.items || []
  } catch (e) {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 14px;
}
.page-title {
  font-size: 17px;
  font-weight: 700;
}
.page-sub {
  font-size: 12px;
  color: #909399;
}
.query-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.q-label {
  font-size: 13px;
  color: #606266;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 10px;
}
.acc-ic {
  color: #409eff;
  vertical-align: -2px;
  margin-right: 4px;
}
.acc-name {
  font-size: 13px;
}
.acc-day {
  font-size: 11px;
  color: #e6a23c;
  margin-left: 4px;
}
.cell-btn {
  border: none;
  background: transparent;
  color: #409eff;
  cursor: pointer;
  font-size: 13px;
  padding: 2px 6px;
  border-radius: 4px;
}
.cell-btn:hover {
  background: #ecf5ff;
}
.cell-zero {
  color: #c0c4cc;
}
.cell-total {
  color: #303133;
}
.exp {
  color: #f56c6c;
}
.detail-total {
  float: left;
  font-size: 13px;
  color: #606266;
  line-height: 32px;
}
.detail-total b {
  color: #f56c6c;
}
</style>
