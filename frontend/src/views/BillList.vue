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
            </div>
            <div class="bill-right">
              <span class="bill-amount" :class="b.type === 'income' ? 'money-income' : 'money-expense'">
                {{ b.type === 'income' ? '+' : '-' }}¥{{ formatMoney(b.amount) }}
              </span>
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

    <BillFormDialog v-model="editVisible" :bill="editingBill" :categories="allCategories" :accounts="accounts" @saved="load(true)" />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { accountApi, billApi, categoryApi } from '../api'
import { currentMonth, dateLabel, formatMoney, shiftMonth } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'
import BillFormDialog from '../components/BillFormDialog.vue'

const router = useRouter()

const month = ref(currentMonth())
const filters = reactive({ type: '', category_id: null, keyword: '' })

const bills = ref([])
const allCategories = ref([])
const accounts = ref([])
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

onMounted(async () => {
  const [cats, accs] = await Promise.all([categoryApi.list(), accountApi.list()])
  allCategories.value = cats || []
  accounts.value = accs || []
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
.filter-kw {
  width: 140px;
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
.more {
  text-align: center;
  padding: 8px 0;
}
</style>
