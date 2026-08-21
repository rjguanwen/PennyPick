<template>
  <div class="dashboard">
    <!-- 顶部：问候 + 月份切换 -->
    <div class="head">
      <div>
        <div class="greeting">{{ greeting }}，{{ auth.displayName }}</div>
        <div class="date-today">{{ todayText }}</div>
      </div>
      <div class="month-switch">
        <button type="button" class="pp-tap" @click="month = shiftMonth(month, -1); load()"><el-icon><ArrowLeft /></el-icon></button>
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" class="month-picker" @change="load" />
        <button type="button" class="pp-tap" @click="month = shiftMonth(month, 1); load()"><el-icon><ArrowRight /></el-icon></button>
      </div>
    </div>

    <!-- 还款逾期提醒 -->
    <el-alert
      v-if="overdueAccounts.length"
      type="error"
      :closable="false"
      show-icon
      class="repay-alert"
    >
      <template #title>
        <span>有 {{ overdueAccounts.length }} 个先用后还账户本月未标记还款：</span>
        <el-button link type="danger" size="small" @click="router.push('/repayment')">去处理</el-button>
      </template>
      <div class="repay-alert-list">
        <span v-for="o in overdueAccounts" :key="o.account.id" class="repay-chip">
          {{ o.account.name }}（逾期 {{ o.overdue_by }} 天）
        </span>
      </div>
    </el-alert>

    <!-- 统计卡片 -->
    <div class="stat-grid">
      <div class="pp-card stat-card expense">
        <div class="label">本月支出</div>
        <div class="value money-expense">¥{{ formatMoney(overview.expense_total) }}</div>
      </div>
      <div class="pp-card stat-card income">
        <div class="label">本月收入</div>
        <div class="value money-income">¥{{ formatMoney(overview.income_total) }}</div>
      </div>
      <div class="pp-card stat-card balance">
        <div class="label">结余</div>
        <div class="value" :class="overview.balance >= 0 ? 'money-income' : 'money-expense'">
          ¥{{ formatMoney(overview.balance) }}
        </div>
      </div>
      <div class="pp-card stat-card count">
        <div class="label">账单笔数</div>
        <div class="value">{{ overview.bill_count }}</div>
      </div>
    </div>

    <!-- 预算进度 / 预警 -->
    <div class="pp-card budget-card">
      <div class="budget-head">
        <span class="budget-title"><el-icon><Odometer /></el-icon> 月预算</span>
        <el-tag v-if="budget && budget.set" :type="budgetTagType" size="small">{{ budgetStatusText }}</el-tag>
        <el-tag v-else size="small" type="info">总预算未设置</el-tag>
      </div>
      <template v-if="budget && budget.amount > 0">
        <el-progress
          :percentage="Math.min(Number(budget.used_percent) || 0, 100)"
          :stroke-width="14"
          :color="budgetColor"
          :show-text="false"
          class="budget-bar"
        />
        <div class="budget-info">
          <span>已用 <b>¥{{ formatMoney(overview.expense_total) }}</b> / ¥{{ formatMoney(budget.amount) }}</span>
          <span>{{ (Number(budget.used_percent) || 0).toFixed(1) }}%</span>
        </div>
        <el-alert
          v-if="budget.status === 'warning'"
          title="预算预警：本月支出已超过预警线，注意控制！"
          type="warning"
          :closable="false"
          show-icon
        />
        <el-alert
          v-else-if="budget.status === 'exceeded'"
          title="本月支出已超出预算！"
          type="error"
          :closable="false"
          show-icon
        />
      </template>
      <div v-else class="budget-empty">
        <span>尚未设置本月总预算</span>
        <el-button link type="primary" size="small" @click="router.push('/budget')">去设置</el-button>
      </div>

      <!-- 分类预算 -->
      <template v-if="categoryBudgets.length">
        <div class="cb-divider">
          <span class="cb-title">分类预算</span>
          <el-button link type="primary" size="small" @click="router.push('/budget')">管理</el-button>
        </div>
        <div v-for="item in categoryBudgets" :key="item.category.id" class="cb-item">
          <CatIcon :icon="item.category.icon" :color="item.category.color" :size="14" />
          <div class="cb-main">
            <div class="cb-head">
              <span class="cb-name">{{ item.category.name }}</span>
              <span class="cb-text">¥{{ formatMoney(item.used) }} / ¥{{ formatMoney(item.budget.amount) }}</span>
              <span class="cb-pct" :class="{ warn: item.budget.status !== 'normal' }">
                {{ (Number(item.budget.used_percent) || 0).toFixed(1) }}%
              </span>
              <el-tag size="small" :type="cbTagType(item.budget)" class="cb-tag">{{ cbStatusText(item.budget) }}</el-tag>
            </div>
            <el-progress
              :percentage="Math.min(Number(item.budget.used_percent) || 0, 100)"
              :stroke-width="6"
              :color="cbColor(item.budget)"
              :show-text="false"
              class="cb-bar"
            />
          </div>
        </div>
      </template>
    </div>

    <!-- 快捷操作 -->
    <div class="quick-actions">
      <el-button type="primary" size="large" class="action-record" @click="router.push('/record')">
        <el-icon><CirclePlusFilled /></el-icon>记一笔
      </el-button>
      <el-button size="large" @click="router.push('/bills')"><el-icon><Tickets /></el-icon>账单</el-button>
      <el-button size="large" @click="router.push('/stats')"><el-icon><DataAnalysis /></el-icon>统计</el-button>
    </div>

    <!-- 最近账单 -->
    <div class="pp-card recent-card">
      <div class="recent-head">
        <span class="budget-title"><el-icon><Clock /></el-icon> 最近账单</span>
        <el-button link type="primary" @click="router.push('/bills')">查看全部</el-button>
      </div>
      <el-skeleton v-if="loading && !recentBills.length" :rows="4" animated />
      <template v-else>
        <div v-for="b in recentBills" :key="b.id" class="bill-item pp-tap" @click="openEdit(b)">
          <CatIcon :icon="b.category?.icon" :color="b.category?.color" :size="18" />
          <div class="bill-main">
            <div class="bill-name">{{ b.category?.name || '未知' }}</div>
            <div class="bill-note">{{ b.note || b.occurred_at.slice(5, 16) }}</div>
          </div>
          <div class="bill-right">
            <span :class="b.type === 'income' ? 'money-income' : 'money-expense'">
              {{ b.type === 'income' ? '+' : '-' }}¥{{ formatMoney(b.amount) }}
            </span>
          </div>
        </div>
        <el-empty v-if="!recentBills.length" description="还没有账单，记一笔吧~" :image-size="70" />
      </template>
    </div>

    <BillFormDialog
      v-model="editVisible"
      :bill="editingBill"
      :categories="categories"
      :accounts="accounts"
      @saved="load"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { accountApi, billApi, budgetApi, categoryApi, repaymentApi, statsApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { currentMonth, formatMoney, shiftMonth } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'
import BillFormDialog from '../components/BillFormDialog.vue'

const router = useRouter()
const auth = useAuthStore()

const month = ref(currentMonth())
const overview = ref({ expense_total: 0, income_total: 0, balance: 0, bill_count: 0, budget: null })
const recentBills = ref([])
const categories = ref([])
const accounts = ref([])
const loading = ref(false)
const overdueAccounts = ref([])

// 首页展示的分类预算（仅已设置预算的支出分类）
const categoryBudgets = ref([])

function cbStatusText(b) {
  const map = { normal: '正常', warning: '预警中', exceeded: '已超支' }
  return map[b.status] || ''
}
function cbTagType(b) {
  const map = { normal: 'success', warning: 'warning', exceeded: 'danger' }
  return map[b.status] || 'info'
}
function cbColor(b) {
  const map = { exceeded: '#f56c6c', warning: '#e6a23c', normal: '#409eff' }
  return map[b.status] || '#c0c4cc'
}

const budget = computed(() => overview.value.budget)
const budgetStatusText = computed(() => {
  if (!budget.value) return ''
  const map = { normal: '正常', warning: '预警', exceeded: '已超支', none: '未设置' }
  return map[budget.value.status] || ''
})
const budgetTagType = computed(() => {
  const map = { normal: 'success', warning: 'warning', exceeded: 'danger' }
  return map[budget.value?.status] || 'info'
})
const budgetColor = computed(() => {
  const s = budget.value?.status
  if (s === 'exceeded') return '#f56c6c'
  if (s === 'warning') return '#e6a23c'
  return '#409eff'
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})
const todayText = computed(() => {
  const d = new Date()
  const week = ['日', '一', '二', '三', '四', '五', '六']
  return `${d.getMonth() + 1}月${d.getDate()}日 星期${week[d.getDay()]}`
})

const editVisible = ref(false)
const editingBill = ref(null)

function openEdit(bill) {
  editingBill.value = bill
  editVisible.value = true
}

async function load() {
  loading.value = true
  try {
    const [ov, bills, cats, accs, cbs] = await Promise.all([
      statsApi.overview(month.value),
      billApi.list({ page: 1, page_size: 8 }),
      categoryApi.list(),
      accountApi.list(),
      budgetApi.categories(month.value),
    ])
    overview.value = ov
    recentBills.value = bills.items || []
    categories.value = cats || []
    accounts.value = accs || []
    // 仅展示已设置预算的支出分类
    categoryBudgets.value = (cbs || []).filter((i) => i.budget)
  } catch (e) {
    ElMessage.error('加载失败')
  } finally {
    loading.value = false
  }
  // 还款逾期提醒（针对当前真实月份）
  try {
    const reps = (await repaymentApi.list(currentMonth())) || []
    overdueAccounts.value = reps.filter((i) => i.overdue)
  } catch (e) {
    overdueAccounts.value = []
  }
}

onMounted(load)
</script>

<style scoped>
.dashboard {
  max-width: 1080px;
  margin: 0 auto;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.greeting {
  font-size: 18px;
  font-weight: 700;
  color: #303133;
}
.date-today {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
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
.repay-alert {
  margin-bottom: 14px;
}
.repay-alert-list {
  margin-top: 6px;
}
.repay-chip {
  margin-right: 12px;
  font-size: 13px;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 14px;
}
.stat-card .label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 8px;
}
.stat-card .value {
  font-size: 20px;
  font-weight: 700;
  word-break: break-all;
}
.budget-card {
  margin-bottom: 14px;
}
.budget-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.budget-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
}
.budget-bar {
  margin-bottom: 8px;
}
.budget-info {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: #909399;
  margin-bottom: 10px;
}
.budget-empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #c0c4cc;
  font-size: 13px;
  padding: 6px 0;
}
/* 分类预算 */
.cb-divider {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 14px 0 4px;
  padding-top: 12px;
  border-top: 1px dashed #ebeef5;
}
.cb-title {
  font-size: 13px;
  font-weight: 600;
  color: #606266;
}
.cb-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
}
.cb-main {
  flex: 1;
  min-width: 0;
}
.cb-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.cb-name {
  font-size: 13px;
  color: #303133;
  min-width: 32px;
}
.cb-text {
  font-size: 12px;
  color: #909399;
}
.cb-pct {
  font-size: 12px;
  font-weight: 600;
  color: #909399;
}
.cb-pct.warn {
  color: #e6a23c;
}
.cb-tag {
  margin-left: auto;
}
.cb-bar {
  width: 100%;
}
.quick-actions {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}
.action-record {
  flex: 1.2;
}
.quick-actions .el-button {
  flex: 1;
}
.recent-card {
  margin-bottom: 14px;
}
.recent-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.bill-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 4px;
  border-bottom: 1px solid #f2f3f5;
  cursor: pointer;
}
.bill-item:last-child {
  border-bottom: none;
}
.bill-main {
  flex: 1;
  min-width: 0;
}
.bill-name {
  font-size: 15px;
  color: #303133;
}
.bill-note {
  font-size: 12px;
  color: #c0c4cc;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bill-right {
  font-size: 15px;
}
</style>
