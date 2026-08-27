<template>
  <div class="bill-list">
    <!-- 月份切换 -->
    <div class="head">
      <div class="month-switch">
        <button type="button" class="pp-tap" @click="changeMonth(-1)"><el-icon><ArrowLeft /></el-icon></button>
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" class="month-picker" @change="resetAndLoad" />
        <button type="button" class="pp-tap" @click="changeMonth(1)"><el-icon><ArrowRight /></el-icon></button>
      </div>
      <div class="head-right">
        <div class="head-total">
          <span>支出 <b class="money-expense">¥{{ formatMoney(monthExpense) }}</b></span>
          <span>收入 <b class="money-income">¥{{ formatMoney(monthIncome) }}</b></span>
        </div>
        <el-button type="primary" :icon="Plus" @click="router.push('/record')">记一笔</el-button>
      </div>
    </div>

    <!-- 筛选 -->
    <div class="filters">
      <el-radio-group v-model="filters.type" size="small" @change="resetAndLoad">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="expense">支出</el-radio-button>
        <el-radio-button value="income">收入</el-radio-button>
      </el-radio-group>
      <el-select v-model="filters.category_id" placeholder="分类" clearable size="small" class="filter-cat" @change="resetAndLoad">
        <el-option v-for="c in allCategories" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-select v-model="filters.account_id" placeholder="账户" clearable size="small" class="filter-acc" @change="resetAndLoad">
        <el-option v-for="a in accounts" :key="a.id" :label="a.name" :value="a.id" />
      </el-select>
      <el-select v-model="filters.tag_id" placeholder="标签" clearable size="small" class="filter-tag" @change="resetAndLoad">
        <el-option v-for="t in allTags" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>
      <el-input v-model="filters.keyword" placeholder="搜备注" size="small" clearable class="filter-kw" @keyup.enter="resetAndLoad" @clear="resetAndLoad" />
      <el-button type="primary" size="small" @click="resetAndLoad">查询</el-button>
    </div>

    <!-- 账单分组列表 -->
    <div class="pp-card list-card">
      <el-skeleton v-if="loading && !grouped.length" :rows="8" animated />
      <template v-else>
        <template v-for="group in grouped" :key="group.date">
          <div class="day-head">
            <span class="day-label">{{ dateLabel(group.date) }} <span class="day-date">{{ group.date.slice(5) }}</span></span>
            <span class="day-total">
              <span v-if="group.expense > 0" class="money-expense">支 ¥{{ formatMoney(group.expense) }}</span>
              <span v-if="group.income > 0" class="money-income">收 ¥{{ formatMoney(group.income) }}</span>
            </span>
          </div>
          <div v-for="b in group.items" :key="b.id" class="bill-item pp-tap" @click="openEdit(b)">
            <CatIcon :icon="b.category?.icon" :color="b.category?.color" :size="18" />
            <div class="bill-main">
              <div class="bill-name">{{ b.category?.name || '未知' }}</div>
              <div class="bill-note">{{ b.note || '' }}<span v-if="b.account" class="bill-acc">{{ b.account.name }}</span></div>
              <div v-if="b.tags && b.tags.length" class="bill-tags">
                <el-tag v-for="t in b.tags" :key="t.id" size="small" type="info" effect="plain" class="bill-tag">{{ t.name }}</el-tag>
              </div>
            </div>
            <div class="bill-right">
              <el-tag v-if="b.type === 'expense' && b.refund_amount > 0" size="small" type="success" effect="plain" class="refund-tag">
                退 ¥{{ formatMoney(b.refund_amount) }}
              </el-tag>
              <span class="bill-amount" :class="b.type === 'income' ? 'money-income' : 'money-expense'">
                {{ b.type === 'income' ? '+' : '-' }}¥{{ formatMoney(b.amount) }}
              </span>
              <el-button v-if="b.type === 'expense'" link type="warning" size="small" class="refund-btn" @click.stop="openRefund(b)">
                {{ b.refund_amount > 0 ? '改退款' : '退款' }}
              </el-button>
              <el-button link type="danger" size="small" class="del-btn" @click.stop="remove(b)">删除</el-button>
            </div>
          </div>
        </template>
        <el-empty v-if="!grouped.length && !loading" description="本月暂无账单" :image-size="80" />
      </template>
      <div v-if="hasMore" class="more">
        <el-button link type="primary" :loading="loading" @click="loadMore">加载更多</el-button>
      </div>
    </div>

    <BillFormDialog v-model="editVisible" :bill="editingBill" :categories="allCategories" :accounts="accounts" :tags="allTags" @saved="load(true)" />

    <!-- 退款登记弹窗 -->
    <el-dialog v-model="refundVisible" title="登记退款" width="min(380px, 92vw)" :close-on-click-modal="false">
      <div v-if="refundBill" class="refund-info">
        原支出：{{ refundBill.category?.name || '未知分类' }} · ¥{{ formatMoney(refundBill.amount) }}
      </div>
      <el-form label-position="top">
        <el-form-item label="退款金额（填 0 表示撤销退款）">
          <el-input-number
            v-model="refundForm.amount"
            :min="0"
            :max="Number(refundBill?.amount || 0)"
            :precision="2"
            :step="1"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="退款日期">
          <el-date-picker
            v-model="refundForm.date"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="选择退款日期"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundVisible = false">取消</el-button>
        <el-button type="primary" :loading="refundSaving" @click="saveRefund">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { accountApi, billApi, categoryApi, tagApi } from '../api'
import { currentMonth, dateLabel, formatMoney, nowDate, shiftMonth } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'
import BillFormDialog from '../components/BillFormDialog.vue'

const router = useRouter()

const month = ref(currentMonth())
const filters = reactive({ type: '', category_id: null, account_id: null, tag_id: null, keyword: '' })

const bills = ref([])
const allCategories = ref([])
const accounts = ref([])
const allTags = ref([])
const page = ref(1)
const pageSize = 50
const total = ref(0)
const loading = ref(false)

const editVisible = ref(false)
const editingBill = ref(null)

const monthExpense = computed(() =>
  bills.value.filter((b) => b.type === 'expense').reduce((s, b) => s + Number(b.amount), 0),
)
const monthIncome = computed(() =>
  bills.value.filter((b) => b.type === 'income').reduce((s, b) => s + Number(b.amount), 0),
)

const grouped = computed(() => {
  const map = {}
  for (const b of bills.value) {
    const date = b.occurred_at.slice(0, 10)
    if (!map[date]) map[date] = { date, items: [], expense: 0, income: 0 }
    map[date].items.push(b)
    if (b.type === 'expense') map[date].expense += Number(b.amount)
    else map[date].income += Number(b.amount)
  }
  return Object.values(map)
})

const hasMore = computed(() => bills.value.length < total.value)

function changeMonth(delta) {
  month.value = shiftMonth(month.value, delta)
  resetAndLoad()
}

function buildParams(pageNum) {
  return {
    month: month.value,
    type: filters.type || undefined,
    category_id: filters.category_id || undefined,
    account_id: filters.account_id || undefined,
    tag_id: filters.tag_id || undefined,
    keyword: filters.keyword || undefined,
    page: pageNum,
    page_size: pageSize,
  }
}

async function load(reset) {
  if (reset) page.value = 1
  loading.value = true
  try {
    const data = await billApi.list(buildParams(page.value))
    if (reset || page.value === 1) {
      bills.value = data.items || []
    } else {
      bills.value = bills.value.concat(data.items || [])
    }
    total.value = data.total || 0
  } catch (e) {
    // 已由拦截器提示
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  page.value = 1
  load(true)
}

function loadMore() {
  page.value += 1
  load(false)
}

function openEdit(bill) {
  editingBill.value = bill
  editVisible.value = true
}

async function remove(bill) {
  try {
    await ElMessageBox.confirm('确定删除这笔账单吗？', '删除账单', { type: 'warning' })
  } catch (e) {
    return
  }
  await billApi.remove(bill.id)
  ElMessage.success('已删除')
  load(true)
}

const refundVisible = ref(false)
const refundBill = ref(null)
const refundSaving = ref(false)
const refundForm = reactive({ amount: 0, date: '' })

function openRefund(bill) {
  refundBill.value = bill
  refundForm.amount = Number(bill.refund_amount || 0)
  refundForm.date = bill.refunded_at ? bill.refunded_at.slice(0, 10) : nowDate()
  refundVisible.value = true
}

async function saveRefund() {
  const b = refundBill.value
  if (!b) return
  if (refundForm.amount < 0 || refundForm.amount > Number(b.amount)) {
    ElMessage.warning(`退款金额需在 0 与 ${formatMoney(b.amount)} 之间`)
    return
  }
  if (refundForm.amount > 0 && !refundForm.date) {
    ElMessage.warning('请选择退款日期')
    return
  }
  refundSaving.value = true
  try {
    const payload = {
      type: b.type,
      amount: Number(b.amount),
      category_id: b.category_id,
      account_id: b.account_id || null,
      occurred_at: (b.occurred_at || '').slice(0, 16),
      note: b.note || '',
      tag_ids: (b.tags || []).map((t) => t.id),
      refund_amount: refundForm.amount,
      refunded_at: refundForm.amount > 0 ? refundForm.date : '',
    }
    await billApi.update(b.id, payload)
    ElMessage.success(refundForm.amount > 0 ? '退款已登记' : '已撤销退款')
    refundVisible.value = false
    load(true)
  } catch (e) {
    // 拦截器已提示
  } finally {
    refundSaving.value = false
  }
}

onMounted(async () => {
  const [cats, accs, tags] = await Promise.all([categoryApi.list(), accountApi.list(), tagApi.list()])
  allCategories.value = cats || []
  accounts.value = accs || []
  allTags.value = tags || []
  load(true)
})
</script>

<style scoped>
.bill-list {
  max-width: 720px;
  margin: 0 auto;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
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
.head-total {
  display: flex;
  gap: 16px;
  font-size: 13px;
  color: #909399;
}
.head-right {
  display: flex;
  align-items: center;
  gap: 16px;
}
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.filter-cat {
  width: 110px;
}
.filter-acc {
  width: 110px;
}
.filter-tag {
  width: 120px;
}
.filter-kw {
  width: 140px;
}
.bill-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 3px;
}
.bill-tag {
  border-radius: 6px;
}
.list-card {
  padding: 8px 16px;
}
.day-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0 4px;
}
.day-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.day-date {
  font-size: 12px;
  color: #c0c4cc;
  font-weight: 400;
}
.day-total {
  font-size: 12px;
  color: #909399;
  display: flex;
  gap: 8px;
}
.bill-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
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
.bill-acc {
  margin-left: 6px;
  background: #f2f3f5;
  padding: 1px 6px;
  border-radius: 6px;
}
.bill-right {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
}
.del-btn {
  display: none;
}
.bill-item:hover .del-btn {
  display: inline-flex;
}
.refund-btn {
  display: none;
}
.bill-item:hover .refund-btn {
  display: inline-flex;
}
.refund-tag {
  border-radius: 6px;
  flex-shrink: 0;
}
.refund-info {
  font-size: 13px;
  color: #909399;
  margin-bottom: 12px;
}
.more {
  text-align: center;
  padding: 8px 0;
}
</style>
