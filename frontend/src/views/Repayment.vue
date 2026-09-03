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

    <!-- 本期还款汇总：醒目展示还需还款金额 -->
    <div v-if="items.length" class="repay-total" :class="{ cleared: repaySummary.remain <= 0 }">
      <div class="rt-main">
        <div class="rt-label">还需还款 · {{ monthLabel }}</div>
        <div class="rt-amount">¥{{ formatMoney(repaySummary.remain) }}</div>
        <div v-if="repaySummary.remain <= 0 && repaySummary.due > 0" class="rt-clear">本期已全部还清，无需再还款</div>
        <div v-else-if="repaySummary.remain <= 0" class="rt-clear">本期无待还账单，无需还款</div>
      </div>
      <div class="rt-sub">
        <div>本期合计应还 <b>¥{{ formatMoney(repaySummary.due) }}</b></div>
        <div>已还金额 <b>¥{{ formatMoney(repaySummary.paid) }}</b></div>
      </div>
    </div>

    <!-- 还款状态需要重新确认（已标记还款后账期内补录了新账单） -->
    <el-alert
      v-if="reconfirmList.length"
      type="warning"
      :closable="false"
      show-icon
      class="overdue-alert"
      :title="`有 ${reconfirmList.length} 个账户已标记还款但账期内新增了账单，请重新标记还款情况`"
    >
      <div class="overdue-list">
        <span v-for="o in reconfirmList" :key="o.account.id" class="overdue-chip">
          {{ o.account.name }}（应还 ¥{{ formatMoney(o.month_expense) }} · 已还 ¥{{ formatMoney(o.amount) }}）
        </span>
      </div>
    </el-alert>

    <!-- 还款状态提醒 -->
    <el-alert
      v-if="overdueList.length"
      type="error"
      :closable="false"
      show-icon
      class="overdue-alert"
      :title="`有 ${overdueList.length} 个账户本期尚未标记还款`"
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
    <el-alert v-else-if="repaidCount === creditCount" type="success" :closable="false" show-icon title="本期所有信用账户均已标记还款" />
    <el-alert v-else-if="pendingCount === 0 && repaidCount > 0" type="success" :closable="false" show-icon title="本期先用后还账户均已还清" />
    <el-alert v-else-if="pendingCount === 0" type="success" :closable="false" show-icon title="本期先用后还账户均无支出，无需还款" />
    <el-alert v-else type="warning" :closable="false" show-icon :title="`本期还有 ${pendingCount} 个账户待还款`" />

    <!-- 账户列表 -->
    <div v-loading="loading" class="list">
      <div v-for="item in items" :key="item.account.id" class="repay-item" :class="{ overdue: item.overdue }">
        <div class="acc-icon"><el-icon :size="24"><component :is="item.account.icon || 'Wallet'" /></el-icon></div>
        <div class="acc-info">
          <div class="acc-name">
            <span v-if="item.has_expense" class="acc-link" @click="showBills(item)">{{ item.account.name }}</span>
            <template v-else>{{ item.account.name }}</template>
          </div>
          <div class="acc-due">本期账单 {{ fmtBillingRange(item) }}</div>
          <div class="acc-due">应还 ¥{{ formatMoney(item.month_expense) }} · 每月 {{ item.account.repay_day }} 日前还款</div>
        </div>
        <div class="acc-status">
          <el-tag v-if="item.repaid && item.needs_reconfirm" type="warning" effect="dark">需重新确认</el-tag>
          <el-tag v-else-if="item.repaid && item.status === 'partial'" type="warning" effect="light">部分还款</el-tag>
          <el-tag v-else-if="item.repaid" type="success" effect="light">已还款</el-tag>
          <el-tag v-else-if="item.overdue" type="danger" effect="light">逾期 {{ item.overdue_by }} 天</el-tag>
          <el-tag v-else-if="!item.has_expense" type="info" effect="plain">本期无支出</el-tag>
          <el-tag v-else type="info" effect="plain">待还款</el-tag>
        </div>
        <div class="acc-time" v-if="item.repaid">
          <div class="repaid-at">{{ fmtTime(item.repaid_at) }}<span v-if="item.amount"> · ¥{{ formatMoney(item.amount) }}</span></div>
          <div class="repaid-note" v-if="item.note">{{ item.note }}</div>
        </div>
        <div class="acc-ops">
          <el-button v-if="item.repaid && item.needs_reconfirm" link type="warning" size="small" @click="openMark(item)">重新标记</el-button>
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
        本期账单（{{ current ? fmtBillingRange(current) : '' }}）应还 ¥{{ current ? formatMoney(current.month_expense) : '0.00' }}。实际还款大于应还将自动补录差额账单；小于则标记为部分还款。
      </div>
      <el-input v-model="markForm.note" placeholder="备注（可选），如 还款渠道" maxlength="255" clearable class="mark-note" />
      <template #footer>
        <el-button @click="markVisible = false">取消</el-button>
        <el-button type="primary" :loading="marking" @click="doMark">确认已还款</el-button>
      </template>
    </el-dialog>

    <!-- 账期账单明细 -->
    <el-dialog v-model="detailVisible" :title="`${detailAccName} · 本期账单明细`" width="660px" top="6vh">
      <div v-loading="detailLoading">
        <template v-if="detailData">
          <div class="billing-summary">
            账单周期：{{ shortDate(detailData.billing_start) }} ~ {{ shortDate(detailData.billing_end) }}
            <span class="b-sum-exp">支出 ¥{{ formatMoney(detailData.expense_total) }}</span>
            <span class="b-sum-inc">收入 ¥{{ formatMoney(detailData.income_total) }}</span>
          </div>
          <el-table :data="detailData.items" size="small" style="width: 100%">
            <el-table-column label="日期" width="120">
              <template #default="{ row }">{{ (row.occurred_at || '').slice(0, 10) }}</template>
            </el-table-column>
            <el-table-column label="分类" width="130">
              <template #default="{ row }">{{ row.category ? row.category.name : '-' }}</template>
            </el-table-column>
            <el-table-column label="金额">
              <template #default="{ row }">
                <span :class="row.type === 'income' ? 'money-income' : 'money-expense'">
                  {{ row.type === 'income' ? '+' : '-' }}¥{{ formatMoney(row.amount) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="备注" min-width="130">
              <template #default="{ row }">{{ row.note }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!detailData.items.length" description="该账期暂无账单" :image-size="50" />
        </template>
      </div>
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

const detailVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref(null)
const detailAccName = ref('')

const monthLabel = computed(() => {
  const [y, m] = month.value.split('-')
  return `${y}年${Number(m)}月`
})
const creditCount = computed(() => items.value.length)
const repaidCount = computed(() => items.value.filter((i) => i.repaid).length)
const pendingCount = computed(() => items.value.filter((i) => !i.repaid && i.has_expense).length)
const noExpenseCount = computed(() => items.value.filter((i) => !i.has_expense).length)
const overdueList = computed(() => items.value.filter((i) => i.overdue))
const reconfirmList = computed(() => items.value.filter((i) => i.needs_reconfirm))

// 本期还款汇总：合计应还 = 各账户账期应还之和；还需还款 = 应还 - 已还（未标记视为 0）
const repaySummary = computed(() => {
  const r2 = (v) => Math.round((Number(v) || 0) * 100) / 100
  let due = 0
  let paid = 0
  let remain = 0
  for (const it of items.value) {
    const exp = r2(it.month_expense)
    const repaid = it.repaid ? r2(it.amount) : 0
    due += exp
    paid += repaid
    remain += exp > repaid ? exp - repaid : 0
  }
  return { due: r2(due), paid: r2(paid), remain: r2(remain) }
})

function fmtTime(t) {
  if (!t) return ''
  return t.slice(0, 10)
}

function shortDate(d) {
  if (!d) return ''
  const parts = d.split('-')
  if (parts.length < 3) return d
  return `${Number(parts[1])}月${Number(parts[2])}日`
}

function fmtBillingRange(item) {
  if (!item || !item.billing_start || !item.billing_end) return ''
  return `${shortDate(item.billing_start)} ~ ${shortDate(item.billing_end)}`
}

function changeMonth(delta) {
  month.value = shiftMonth(month.value, delta)
  load()
}

async function load() {
  loading.value = true
  try {
    items.value = (await repaymentApi.list(month.value)) || []
    // 校验：已标记还款但账期内补录了新账单（应还 > 已还）时提示重新标记
    const stale = items.value.filter((i) => i.needs_reconfirm)
    if (stale.length) {
      ElMessage.warning(`有 ${stale.length} 个账户已标记还款但账期内新增了账单，应还金额已变化，请重新标记还款情况`)
    }
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

async function showBills(item) {
  detailAccName.value = item.account.name
  detailVisible.value = true
  detailLoading.value = true
  detailData.value = null
  try {
    detailData.value = await repaymentApi.bills(month.value, item.account.id)
  } catch (e) {
    // 拦截器已提示
  } finally {
    detailLoading.value = false
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
/* 本期还款汇总条 */
.repay-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 14px;
  padding: 16px 22px;
  border-radius: 12px;
  color: #fff;
  background: linear-gradient(135deg, #ff9a3d, #f56c6c);
  box-shadow: 0 4px 14px rgba(245, 108, 108, 0.3);
}
.repay-total.cleared {
  background: linear-gradient(135deg, #67c23a, #95d475);
  box-shadow: 0 4px 14px rgba(103, 194, 58, 0.3);
}
.rt-main {
  flex: 1;
  min-width: 0;
}
.rt-label {
  font-size: 13px;
  opacity: 0.95;
}
.rt-amount {
  margin-top: 4px;
  font-size: 34px;
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: 0.5px;
  font-variant-numeric: tabular-nums;
}
.rt-clear {
  margin-top: 2px;
  font-size: 12px;
  opacity: 0.95;
}
.rt-sub {
  text-align: right;
  font-size: 13px;
  line-height: 2;
  opacity: 0.98;
  white-space: nowrap;
}
.rt-sub b {
  font-size: 16px;
  font-weight: 700;
}
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
.acc-link {
  color: #409eff;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 3px;
}
.acc-link:hover {
  color: #66b1ff;
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
.billing-summary {
  font-size: 13px;
  color: #606266;
  margin-bottom: 10px;
}
.b-sum-exp {
  color: #f56c6c;
  margin-left: 12px;
  font-weight: 600;
}
.b-sum-inc {
  color: #67c23a;
  margin-left: 12px;
  font-weight: 600;
}
.money-expense {
  color: #f56c6c;
}
.money-income {
  color: #67c23a;
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
