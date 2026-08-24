<template>
  <div class="repayment-page">
    <!-- 顶部：月份切换 + 统计 -->
    <div class="page-head">
      <div class="month-switch">
        <el-button circle text size="small" @click="changeMonth(-1)"><el-icon><ArrowLeft /></el-icon></el-button>
        <span class="month-text">{{ monthLabel }}</span>
        <el-button circle text size="small" @click="changeMonth(1)"><el-icon><ArrowRight /></el-icon></el-button>
      </div>
      <div class="head-stats">
        <span>已还 <b class="ok">{{ repaidCount }}</b></span>
        <span class="sep">/</span>
        <span>待还 <b class="warn">{{ pendingCount }}</b></span>
        <span class="sep">/</span>
        <span>无支出 <b class="none">{{ noExpenseCount }}</b></span>
      </div>
    </div>

    <!-- 还款状态提醒 -->
    <el-alert
      v-if="overdueList.length"
      type="error"
      :closable="false"
      show-icon
      class="overdue-alert"
      :title="`有 ${overdueList.length} 个账户本月尚未标记还款`"
    >
      <div class="overdue-list">
        <span v-for="o in overdueList" :key="o.account.id" class="overdue-chip">
          {{ o.account.name }}（逾期 {{ o.overdue_by }} 天）
        </span>
      </div>
    </el-alert>
    <el-alert v-else-if="creditCount === 0" type="info" :closable="false" show-icon title="还没有先用后还的账户"
      description="可在「设置 → 账户管理」新建账户时勾选「先用后还」并设置每月还款日，或编辑已有账户。"
    />
    <el-alert v-else-if="repaidCount === creditCount" type="success" :closable="false" show-icon title="本月所有信用账户均已标记还款" />
    <el-alert v-else-if="pendingCount === 0 && repaidCount > 0" type="success" :closable="false" show-icon title="本月先用后还账户均已还清" />
    <el-alert v-else-if="pendingCount === 0" type="success" :closable="false" show-icon title="本月先用后还账户均无支出，无需还款" />
    <el-alert v-else type="warning" :closable="false" show-icon :title="`本月还有 ${pendingCount} 个账户待还款`" />

    <!-- 账户列表 -->
    <div v-loading="loading" class="list">
      <div v-for="item in items" :key="item.account.id" class="repay-item" :class="{ overdue: item.overdue }">
        <div class="acc-icon"><el-icon :size="24"><component :is="item.account.icon || 'Wallet'" /></el-icon></div>
        <div class="acc-info">
          <div class="acc-name">{{ item.account.name }}</div>
          <div class="acc-due">每月 {{ item.account.repay_day }} 日前还款 · 本月支出 ¥{{ formatMoney(item.month_expense) }}</div>
        </div>
        <div class="acc-status">
          <el-tag v-if="item.repaid && item.status === 'partial'" type="warning" effect="light">部分还款</el-tag>
          <el-tag v-else-if="item.repaid" type="success" effect="light">已还款</el-tag>
          <el-tag v-else-if="item.overdue" type="danger" effect="light">逾期 {{ item.overdue_by }} 天</el-tag>
          <el-tag v-else-if="!item.has_expense" type="info" effect="plain">本月无支出</el-tag>
          <el-tag v-else type="info" effect="plain">待还款</el-tag>
        </div>
        <div class="acc-time" v-if="item.repaid">
          <div class="repaid-at">{{ fmtTime(item.repaid_at) }}<span v-if="item.amount"> · ¥{{ formatMoney(item.amount) }}</span></div>
          <div class="repaid-note" v-if="item.note">{{ item.note }}</div>
        </div>
        <div class="acc-ops">
          <el-button v-if="item.repaid" link type="danger" size="small" @click="unmark(item)">取消标记</el-button>
          <el-button v-else-if="item.has_expense" type="primary" size="small" @click="openMark(item)">标记已还款</el-button>
          <span v-else class="no-op">无需还款</span>
        </div>
      </div>
      <el-empty v-if="!loading && creditCount === 0" description="暂无先用后还账户" />
    </div>

    <!-- 标记还款对话框 -->
    <el-dialog v-model="markVisible" title="标记已还款" width="440px">
      <div v-if="current" class="mark-desc">
        确认「{{ current.account.name }}」已完成 <b>{{ monthLabel }}</b> 还款？
      </div>
      <div class="mark-amount">
        <span class="mark-label">实际还款金额</span>
        <el-input-number v-model="markForm.amount" :min="0" :precision="2" :step="10" controls-position="right" style="width: 180px" />
      </div>
      <div class="mark-tip">
        本月账单合计 ¥{{ current ? formatMoney(current.month_expense) : '0.00' }}。实际还款大于合计将自动补录差额账单；小于合计则标记为部分还款。
      </div>
      <el-input v-model="markForm.note" placeholder="备注（可选），如 还款渠道" maxlength="255" clearable class="mark-note" />
      <template #footer>
        <el-button @click="markVisible = false">取消</el-button>
        <el-button type="primary" :loading="marking" @click="doMark">确认已还款</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { repaymentApi } from '../api'
import { currentMonth, formatMoney, shiftMonth } from '../utils/format'

const month = ref(currentMonth())
const items = ref([])
const loading = ref(false)

const markVisible = ref(false)
const marking = ref(false)
const current = ref(null)
const markForm = reactive({ amount: 0, note: '' })

const monthLabel = computed(() => {
  const [y, m] = month.value.split('-')
  return `${y}年${Number(m)}月`
})
const creditCount = computed(() => items.value.length)
const repaidCount = computed(() => items.value.filter((i) => i.repaid).length)
const pendingCount = computed(() => items.value.filter((i) => !i.repaid && i.has_expense).length)
const noExpenseCount = computed(() => items.value.filter((i) => !i.has_expense).length)
const overdueList = computed(() => items.value.filter((i) => i.overdue))

function fmtTime(t) {
  if (!t) return ''
  return t.slice(0, 10)
}

function changeMonth(delta) {
  month.value = shiftMonth(month.value, delta)
  load()
}

async function load() {
  loading.value = true
  try {
    items.value = (await repaymentApi.list(month.value)) || []
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

function openMark(item) {
  current.value = item
  markForm.amount = item.month_expense || 0
  markForm.note = ''
  markVisible.value = true
}

async function doMark() {
  if (markForm.amount == null || markForm.amount < 0) {
    ElMessage.warning('请输入实际还款金额')
    return
  }
  marking.value = true
  try {
    const res = await repaymentApi.mark({
      account_id: current.value.account.id,
      month: month.value,
      amount: markForm.amount,
      note: markForm.note.trim(),
    })
    if (res && res.diff_bill) {
      ElMessage.success(`已标记还款，并自动补录差额账单 ¥${formatMoney(res.diff_amount || 0)}`)
    } else if (res && res.status === 'partial') {
      ElMessage.info('已标记为部分还款，该账户本月尚未还清')
    } else {
      ElMessage.success('已标记还款')
    }
    markVisible.value = false
    load()
  } catch (e) {
    // 拦截器已提示
  } finally {
    marking.value = false
  }
}

async function unmark(item) {
  try {
    await ElMessageBox.confirm(
      `确定取消「${item.account.name}」${monthLabel.value}的还款标记吗？`,
      '取消标记',
      { type: 'warning' }
    )
  } catch (e) {
    return
  }
  try {
    await repaymentApi.unmark(month.value, item.account.id)
    ElMessage.success('已取消标记')
    load()
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
  margin-bottom: 16px;
}
.month-switch {
  display: flex;
  align-items: center;
  gap: 4px;
}
.month-text {
  font-size: 17px;
  font-weight: 700;
  min-width: 108px;
  text-align: center;
}
.head-stats {
  font-size: 14px;
  color: #909399;
}
.head-stats b {
  font-size: 16px;
  margin: 0 2px;
}
.head-stats .ok {
  color: #67c23a;
}
.head-stats .warn {
  color: #e6a23c;
}
.head-stats .none {
  color: #909399;
}
.head-stats .sep {
  margin: 0 6px;
  color: #dcdfe6;
}
.overdue-alert {
  margin-bottom: 14px;
}
.overdue-list {
  margin-top: 6px;
}
.overdue-chip {
  margin-right: 12px;
  font-size: 13px;
}
.list {
  margin-top: 4px;
}
.repay-item {
  display: flex;
  align-items: center;
  gap: 14px;
  background: #fff;
  border-radius: 10px;
  padding: 14px 18px;
  margin-bottom: 10px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  border-left: 4px solid transparent;
}
.repay-item.overdue {
  border-left-color: #f56c6c;
}
.acc-icon {
  font-size: 26px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  border-radius: 10px;
  flex-shrink: 0;
}
.acc-info {
  flex: 1;
  min-width: 0;
}
.acc-name {
  font-size: 15px;
  font-weight: 600;
}
.acc-due {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
.acc-status {
  width: 100px;
  text-align: center;
}
.acc-time {
  width: 150px;
  font-size: 12px;
  color: #909399;
}
.repaid-at {
  color: #67c23a;
}
.repaid-note {
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.acc-ops {
  width: 130px;
  text-align: right;
}
.no-op {
  font-size: 12px;
  color: #c0c4cc;
}
.mark-desc {
  margin-bottom: 14px;
  font-size: 14px;
  color: #606266;
}
.mark-amount {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}
.mark-label {
  font-size: 14px;
  color: #606266;
}
.mark-tip {
  font-size: 12px;
  color: #c0c4cc;
  margin-bottom: 12px;
  line-height: 1.6;
}
.mark-note {
  margin-bottom: 0;
}
</style>
