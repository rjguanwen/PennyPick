<template>
  <div class="budget">
    <!-- 月份切换 + 复制预算 -->
    <div class="head">
      <div class="month-switch">
        <button type="button" class="pp-tap" @click="month = shiftMonth(month, -1); load()"><el-icon><ArrowLeft /></el-icon></button>
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" class="month-picker" @change="load" />
        <button type="button" class="pp-tap" @click="month = shiftMonth(month, 1); load()"><el-icon><ArrowRight /></el-icon></button>
      </div>
      <el-button :icon="CopyDocument" @click="openCopyDialog">复制预算</el-button>
    </div>

    <!-- 总预算：当前月状态 -->
    <div class="pp-card current">
      <div class="current-head">
        <span class="title">总预算 · {{ monthLabel(month) }}</span>
        <el-tag v-if="budget.set" :type="statusTagType" size="small">{{ statusText }}</el-tag>
      </div>
      <template v-if="budget.set && budget.amount > 0">
        <el-progress
          :percentage="Math.min(Number(budget.used_percent) || 0, 100)"
          :stroke-width="16"
          :color="statusColor"
          :text-inside="true"
          :format="() => (Number(budget.used_percent) || 0).toFixed(1) + '%'"
        />
        <div class="current-info">
          <span>已用 ¥{{ formatMoney(overview.expense_total) }} / 预算 ¥{{ formatMoney(budget.amount) }}</span>
          <span v-if="budget.status === 'warning'" class="warn-text">已超过预警线 {{ budget.warn_percent }}%</span>
          <span v-else-if="budget.status === 'exceeded'" class="exceed-text">已超支！</span>
        </div>
      </template>
      <div v-else class="current-empty">本月未设置总预算，设置后按预警线自动提醒</div>
    </div>

    <!-- 总预算设置 -->
    <div class="pp-card form-card">
      <div class="form-title">总预算设置</div>
      <el-form label-position="top">
        <el-form-item label="月预算金额（元）">
          <el-input-number v-model="form.amount" :min="0" :max="999999999" :precision="2" :step="100" controls-position="right" style="width: 100%" placeholder="如 5000" />
        </el-form-item>
        <el-form-item label="预警阈值：已用预算达到该比例时提醒">
          <div class="slider-row">
            <el-slider v-model="form.warn_percent" :min="50" :max="100" :step="5" style="flex: 1" />
            <span class="percent">{{ form.warn_percent }}%</span>
          </div>
        </el-form-item>
        <div class="form-actions">
          <el-button type="primary" :loading="saving" @click="save">保存总预算</el-button>
          <el-button v-if="budget.set" type="danger" plain @click="removeBudget">删除总预算</el-button>
        </div>
      </el-form>
    </div>

    <!-- 分类预算 -->
    <div class="pp-card">
      <div class="current-head">
        <span class="title">分类预算 · {{ monthLabel(month) }}</span>
        <el-tooltip content="为单个分类单独设预算，与总预算独立预警" placement="top">
          <el-icon class="tip-icon"><QuestionFilled /></el-icon>
        </el-tooltip>
      </div>
      <div class="cb-tip">为单个支出分类设置月预算，独立预警（如：餐饮每月 800 元）</div>

      <el-skeleton v-if="cbLoading" :rows="4" animated />
      <el-empty v-else-if="!categoryBudgets.length" description="暂无支出分类" :image-size="60" />
      <div v-else class="cb-list">
        <div v-for="item in categoryBudgets" :key="item.category.id" class="cb-row">
          <CatIcon :icon="item.category.icon" :color="item.category.color" :size="18" />
          <div class="cb-main">
            <div class="cb-head">
              <span class="cb-name">{{ item.category.name }}</span>
              <span class="cb-used">已用 ¥{{ formatMoney(item.used) }}</span>
              <el-tag v-if="item.budget" size="small" :type="cbTagType(item.budget)">{{ cbStatusText(item.budget) }}</el-tag>
              <el-tag v-else size="small" type="info">未设置</el-tag>
            </div>
            <template v-if="item.budget">
              <el-progress
                :percentage="Math.min(Number(item.budget.used_percent) || 0, 100)"
                :stroke-width="8"
                :color="cbColor(item.budget)"
                :show-text="false"
                class="cb-bar"
              />
              <div class="cb-budget-line">
                预算 ¥{{ formatMoney(item.budget.amount) }} · 预警线 {{ item.budget.warn_percent }}%
              </div>
            </template>
          </div>
          <div class="cb-ops">
            <el-button size="small" type="primary" plain @click="openCatBudget(item)">
              {{ item.budget ? '修改' : '设置' }}
            </el-button>
            <el-button v-if="item.budget" size="small" type="danger" plain @click="removeCatBudget(item)">删除</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 历史预算 -->
    <div class="pp-card">
      <div class="form-title">历史总预算</div>
      <el-empty v-if="!history.length" description="暂无历史总预算" :image-size="60" />
      <div v-for="b in history" :key="b.month" class="history-row pp-tap" @click="switchMonth(b.month)">
        <span class="h-month">{{ monthLabel(b.month) }}</span>
        <span class="h-amount">¥{{ formatMoney(b.amount) }}</span>
        <el-tag size="small" :type="historyTagType(b)">{{ historyStatus(b) }}</el-tag>
      </div>
    </div>

    <!-- 复制预算弹窗 -->
    <el-dialog v-model="copyVisible" title="复制预算" width="min(420px, 92vw)" :close-on-click-modal="false">
      <div class="copy-tip">将所选月份的预算（总预算 + 分类预算）复制到 <b>{{ monthLabel(month) }}</b>，目标月已有预算将被覆盖。</div>
      <el-form label-position="top">
        <el-form-item label="选择要复制的月份">
          <el-date-picker v-model="copyFromMonth" type="month" value-format="YYYY-MM" format="YYYY年MM月" :clearable="false" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="copyVisible = false">取消</el-button>
        <el-button type="primary" :loading="copying" @click="doCopyBudget">复制</el-button>
      </template>
    </el-dialog>

    <!-- 分类预算设置弹窗 -->
    <el-dialog
      v-model="cbDialogVisible"
      :title="`设置「${cbForm.category_name}」预算`"
      width="min(440px, 92vw)"
      :close-on-click-modal="false"
    >
      <el-form label-position="top">
        <div class="cb-dialog-info">
          <span>{{ monthLabel(month) }} 已用：<b class="money-expense">¥{{ formatMoney(cbForm.used) }}</b></span>
        </div>
        <el-form-item label="预算金额（元）">
          <el-input-number v-model="cbForm.amount" :min="1" :max="999999999" :precision="2" :step="100" controls-position="right" style="width: 100%" placeholder="如 800" />
        </el-form-item>
        <el-form-item label="预警阈值：已用达到该比例时提醒">
          <div class="slider-row">
            <el-slider v-model="cbForm.warn_percent" :min="50" :max="100" :step="5" style="flex: 1" />
            <span class="percent">{{ cbForm.warn_percent }}%</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cbDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="cbSaving" @click="saveCatBudget">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { budgetApi, statsApi } from '../api'
import { currentMonth, formatMoney, monthLabel, shiftMonth } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'

const month = ref(currentMonth())
const budget = ref({ set: false, amount: 0, warn_percent: 80, used_percent: 0, status: 'none' })
const overview = ref({ expense_total: 0 })
const history = ref([])
const saving = ref(false)

const form = reactive({ amount: null, warn_percent: 80 })

// ===== 总预算状态 =====
const statusText = computed(() => {
  const map = { normal: '正常', warning: '预警中', exceeded: '已超支', none: '未设置' }
  return map[budget.value.status] || ''
})
const statusTagType = computed(() => {
  const map = { normal: 'success', warning: 'warning', exceeded: 'danger', none: 'info' }
  return map[budget.value.status] || 'info'
})
const statusColor = computed(() => {
  const map = { exceeded: '#f56c6c', warning: '#e6a23c', normal: '#409eff' }
  return map[budget.value.status] || '#c0c4cc'
})

const usedMap = reactive({})

function historyStatus(b) {
  const used = usedMap[b.month]
  if (!used) return '未使用'
  const pct = (used / b.amount) * 100
  if (pct >= 100) return '超支'
  if (pct >= b.warn_percent) return '预警'
  return '正常'
}
function historyTagType(b) {
  const used = usedMap[b.month]
  if (!used) return 'info'
  const pct = (used / b.amount) * 100
  if (pct >= 100) return 'danger'
  if (pct >= b.warn_percent) return 'warning'
  return 'success'
}

// ===== 分类预算 =====
const categoryBudgets = ref([])
const cbLoading = ref(false)
const cbDialogVisible = ref(false)
const cbSaving = ref(false)
const cbForm = reactive({
  category_id: null,
  category_name: '',
  used: 0,
  amount: null,
  warn_percent: 80,
})

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

function openCatBudget(item) {
  cbForm.category_id = item.category.id
  cbForm.category_name = item.category.name
  cbForm.used = item.used
  cbForm.amount = item.budget ? item.budget.amount : null
  cbForm.warn_percent = item.budget ? item.budget.warn_percent : 80
  cbDialogVisible.value = true
}

async function saveCatBudget() {
  if (cbForm.amount === null || cbForm.amount === undefined || cbForm.amount <= 0) {
    ElMessage.warning('请输入预算金额')
    return
  }
  cbSaving.value = true
  try {
    await budgetApi.upsertCategory({
      month: month.value,
      category_id: cbForm.category_id,
      amount: cbForm.amount,
      warn_percent: cbForm.warn_percent,
    })
    ElMessage.success('分类预算已保存')
    cbDialogVisible.value = false
    load()
  } finally {
    cbSaving.value = false
  }
}

async function removeCatBudget(item) {
  try {
    await ElMessageBox.confirm(`确定删除「${item.category.name}」的分类预算吗？`, '删除分类预算', { type: 'warning' })
  } catch (e) {
    return
  }
  await budgetApi.removeCategory(month.value, item.category.id)
  ElMessage.success('已删除')
  load()
}

// ===== 数据加载 =====
function switchMonth(m) {
  month.value = m
  load()
}

async function load() {
  cbLoading.value = true
  try {
    const [b, ov, all, cbs] = await Promise.all([
      budgetApi.get(month.value),
      statsApi.overview(month.value),
      budgetApi.all(),
      budgetApi.categories(month.value),
    ])
    // 总预算展示数据以 overview 为准（含 used_percent / status），表单数据以 budgetApi.get 为准
    budget.value = ov?.budget
      ? { ...ov.budget, set: true }
      : { set: false, amount: 0, warn_percent: 80, used_percent: 0, status: 'none' }
    overview.value = ov || { expense_total: 0 }
    history.value = all || []
    form.amount = b?.set ? b.amount : null
    form.warn_percent = b?.set ? b.warn_percent : 80
    categoryBudgets.value = cbs || []
    // 计算历史各月已用支出
    for (const h of history.value) {
      try {
        const o = await statsApi.overview(h.month)
        usedMap[h.month] = o.expense_total
      } catch (e) {}
    }
  } finally {
    cbLoading.value = false
  }
}

async function save() {
  if (form.amount === null || form.amount === undefined) {
    ElMessage.warning('请输入预算金额')
    return
  }
  if (form.amount <= 0) {
    ElMessage.warning('预算金额需大于 0')
    return
  }
  saving.value = true
  try {
    await budgetApi.upsert({
      month: month.value,
      amount: form.amount,
      warn_percent: form.warn_percent,
    })
    ElMessage.success('总预算已保存')
    load()
  } finally {
    saving.value = false
  }
}

async function removeBudget() {
  try {
    await ElMessageBox.confirm('确定删除本月的总预算吗？', '删除总预算', { type: 'warning' })
  } catch (e) {
    return
  }
  await budgetApi.remove(month.value)
  ElMessage.success('已删除')
  load()
}

// ===== 复制预算 =====
const copyVisible = ref(false)
const copyFromMonth = ref('')
const copying = ref(false)

function openCopyDialog() {
  copyFromMonth.value = shiftMonth(month.value, -1)
  copyVisible.value = true
}

async function doCopyBudget() {
  if (!copyFromMonth.value) {
    ElMessage.warning('请选择要复制的月份')
    return
  }
  if (copyFromMonth.value === month.value) {
    ElMessage.warning('源月份不能与当前月份相同')
    return
  }
  copying.value = true
  try {
    const res = await budgetApi.copy({ from_month: copyFromMonth.value, to_month: month.value })
    ElMessage.success(`已复制：总预算${res.total_copied ? '已复制' : '（源无）'}，分类预算 ${res.category_count} 项`)
    copyVisible.value = false
    load()
  } catch (e) {
    // 拦截器已提示
  } finally {
    copying.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.budget {
  max-width: 760px;
  margin: 0 auto;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.copy-tip {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
  margin-bottom: 14px;
}
.copy-tip b {
  color: #409eff;
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
.current,
.form-card {
  margin-bottom: 12px;
}
.current-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.title,
.form-title {
  font-size: 15px;
  font-weight: 600;
}
.tip-icon {
  color: #c0c4cc;
  cursor: help;
}
.current-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: #909399;
  margin-top: 10px;
}
.warn-text {
  color: #e6a23c;
}
.exceed-text {
  color: #f56c6c;
  font-weight: 600;
}
.current-empty {
  color: #c0c4cc;
  font-size: 13px;
  padding: 8px 0;
}
.slider-row {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
}
.percent {
  width: 48px;
  text-align: right;
  font-weight: 600;
  color: #409eff;
}
.form-actions {
  display: flex;
  gap: 10px;
}
/* 分类预算 */
.cb-tip {
  font-size: 12px;
  color: #c0c4cc;
  margin-bottom: 12px;
}
.cb-list {
  display: flex;
  flex-direction: column;
}
.cb-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #f2f3f5;
}
.cb-row:last-child {
  border-bottom: none;
}
.cb-main {
  flex: 1;
  min-width: 0;
}
.cb-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.cb-name {
  font-size: 15px;
  font-weight: 600;
}
.cb-used {
  font-size: 13px;
  color: #909399;
}
.cb-bar {
  margin-top: 8px;
}
.cb-budget-line {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 6px;
}
.cb-ops {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.cb-dialog-info {
  margin-bottom: 14px;
  font-size: 14px;
  color: #606266;
}
.history-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid #f2f3f5;
  cursor: pointer;
}
.history-row:last-child {
  border-bottom: none;
}
.h-month {
  flex: 1;
  font-size: 14px;
}
.h-amount {
  color: #606266;
  font-size: 14px;
}
</style>
