<template>
  <div class="record-page">
    <!-- 类型切换 -->
    <div class="type-switch">
      <button type="button" class="type-btn pp-tap" :class="{ active: form.type === 'expense' }" @click="setType('expense')">
        <el-icon><Top /></el-icon>支出
      </button>
      <button type="button" class="type-btn pp-tap" :class="{ active: form.type === 'income' }" @click="setType('income')">
        <el-icon><Bottom /></el-icon>收入
      </button>
    </div>

    <div class="record-body">
      <!-- 左：金额 + 数字键盘 -->
      <div class="record-left pp-card">
        <div class="amount-area">
          <span class="amount-cur">¥</span>
          <span class="amount-text">{{ displayAmount }}</span>
        </div>
        <div class="keypad">
          <button type="button" class="key pp-tap" @click="inputKey('1')">1</button>
          <button type="button" class="key pp-tap" @click="inputKey('2')">2</button>
          <button type="button" class="key pp-tap" @click="inputKey('3')">3</button>
          <button type="button" class="key pp-tap" @click="inputKey('4')">4</button>
          <button type="button" class="key pp-tap" @click="inputKey('5')">5</button>
          <button type="button" class="key pp-tap" @click="inputKey('6')">6</button>
          <button type="button" class="key pp-tap" @click="inputKey('7')">7</button>
          <button type="button" class="key pp-tap" @click="inputKey('8')">8</button>
          <button type="button" class="key pp-tap" @click="inputKey('9')">9</button>
          <button type="button" class="key pp-tap" @click="inputKey('.')">.</button>
          <button type="button" class="key pp-tap" @click="inputKey('0')">0</button>
          <button class="key back pp-tap" @click="backspace"><el-icon><Delete /></el-icon></button>
        </div>
      </div>

      <!-- 右：分类 + 信息 + 保存 -->
      <div class="record-right">
        <div class="pp-card cats-card">
          <div class="block-title">选择分类</div>
          <div class="cats">
            <button
              v-for="cat in sortedCategories"
              :key="cat.id"
              class="cat pp-tap"
              :class="{ selected: form.category_id === cat.id }"
              @click="form.category_id = cat.id"
            >
              <CatIcon :icon="cat.icon" :color="cat.color" :size="20" />
              <span class="cat-name">{{ cat.name }}</span>
              <span v-if="form.category_id === cat.id" class="cat-check"><el-icon><Check /></el-icon></span>
            </button>
            <button type="button" class="cat pp-tap" @click="router.push('/categories')">
              <CatIcon icon="Plus" color="#909399" :size="20" />
              <span class="cat-name">管理</span>
            </button>
          </div>
        </div>

        <div class="pp-card info-card">
          <div class="info-row">
            <span class="info-label">日期</span>
            <el-date-picker v-model="form.occurred_at" type="date" value-format="YYYY-MM-DD" format="YYYY-MM-DD" placeholder="选择日期" :clearable="false" style="width: 220px" />
          </div>
          <div class="info-row">
            <span class="info-label">账户</span>
            <el-select v-model="form.account_id" placeholder="选择账户" clearable style="width: 220px">
              <el-option v-for="acc in accounts" :key="acc.id" :label="acc.name" :value="acc.id" />
            </el-select>
          </div>
          <div class="info-row">
            <span class="info-label">备注</span>
            <el-input v-model="form.note" placeholder="备注：买了什么？" maxlength="255" clearable style="width: 220px" @keyup.enter="save" />
          </div>
        </div>

        <div class="save-area">
          <el-switch v-model="continueMode" active-text="保存后继续记" />
          <button type="button" class="save-btn pp-tap" :class="{ income: form.type === 'income' }" :disabled="saving" @click="save">
            {{ saving ? '保存中…' : '保存' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElNotification } from 'element-plus'
import { accountApi, billApi, budgetApi, categoryApi, statsApi } from '../api'
import { formatMoney, nowDate } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'

const router = useRouter()

const amount = ref('')
const saving = ref(false)
const continueMode = ref(false)
const form = reactive({
  type: 'expense',
  category_id: null,
  account_id: null,
  occurred_at: nowDate(),
  note: '',
})

const categories = ref([])
const accounts = ref([])

const displayAmount = computed(() => {
  if (!amount.value) return '0'
  return Number(amount.value).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
})

const sortedCategories = computed(() => {
  return [...categories.value]
    .filter((c) => c.type === form.type)
    .sort((a, b) => (b.recent_count || 0) - (a.recent_count || 0))
})

function setType(t) {
  if (form.type === t) return
  form.type = t
  // 自动选中新类型下最常用的分类，减少点击次数
  const first = sortedCategories.value[0]
  form.category_id = first ? first.id : null
}

function inputKey(k) {
  if (k === '.') {
    if (amount.value.includes('.')) return
    if (amount.value === '') amount.value = '0.'
    else amount.value += '.'
    return
  }
  let next = amount.value === '' ? k : amount.value + k
  if (next.includes('.')) {
    const [int, dec] = next.split('.')
    if (dec.length > 2) return
    if (int.replace(/^0+/, '').length > 9) return
  } else if (next.length > 9) {
    return
  }
  amount.value = next
}

function backspace() {
  amount.value = amount.value.slice(0, -1)
}

async function load() {
  const [cats, accs] = await Promise.all([categoryApi.list(), accountApi.list()])
  categories.value = cats || []
  accounts.value = accs || []
  if (!form.category_id) {
    const first = sortedCategories.value[0]
    if (first) form.category_id = first.id
  }
}

async function save() {
  const val = Number(amount.value || 0)
  if (val <= 0) {
    ElMessage.warning('请输入金额')
    return
  }
  if (!form.category_id) {
    ElMessage.warning('请选择分类')
    return
  }
  saving.value = true
  try {
    await billApi.create({
      type: form.type,
      amount: val,
      category_id: form.category_id,
      account_id: form.account_id,
      occurred_at: form.occurred_at,
      note: form.note,
    })

    checkBudgetWarning()

    if (continueMode.value) {
      amount.value = ''
      form.note = ''
      form.occurred_at = nowDate()
      ElMessage.success('已记账，继续记下一笔')
      load()
    } else {
      ElMessage.success('记账成功')
      router.push('/dashboard')
    }
  } finally {
    saving.value = false
  }
}

async function checkBudgetWarning() {
  try {
    const month = form.occurred_at.slice(0, 7)
    const [ov, cbs] = await Promise.all([statsApi.overview(month), budgetApi.categories(month)])
    // 总预算预警
    if (ov.budget && ov.budget.status === 'warning') {
      ElNotification({
        title: '预算预警',
        message: `本月已使用预算的 ${(Number(ov.budget.used_percent) || 0).toFixed(1)}%，注意控制支出！`,
        type: 'warning',
      })
    } else if (ov.budget && ov.budget.status === 'exceeded') {
      ElNotification({
        title: '预算超支',
        message: `本月支出已超出预算（${(Number(ov.budget.used_percent) || 0).toFixed(1)}%）！`,
        type: 'error',
      })
    }
    // 分类预算预警（当前记账分类）
    if (form.category_id && cbs) {
      const item = cbs.find((c) => c.category.id === form.category_id && c.budget)
      if (item && item.budget) {
        const pct = (Number(item.budget.used_percent) || 0).toFixed(1)
        if (item.budget.status === 'exceeded') {
          ElNotification({
            title: '分类预算超支',
            message: `「${item.category.name}」本月支出已超出预算（${pct}%）！`,
            type: 'error',
          })
        } else if (item.budget.status === 'warning') {
          ElNotification({
            title: '分类预算预警',
            message: `「${item.category.name}」本月已使用预算的 ${pct}%，注意控制！`,
            type: 'warning',
          })
        }
      }
    }
  } catch (e) {
    // 忽略预警检查失败
  }
}

onMounted(load)
</script>

<style scoped>
.record-page {
  max-width: 1000px;
  margin: 0 auto;
}
.type-switch {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.type-btn {
  flex: 1;
  height: 48px;
  border-radius: 10px;
  border: 1px solid #dcdfe6;
  background: #fff;
  font-size: 16px;
  font-weight: 600;
  color: #606266;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}
.type-btn.active.expense {
  background: #fef0f0;
  border-color: #f56c6c;
  color: #f56c6c;
}
.type-btn.active.income {
  background: #f0f9eb;
  border-color: #67c23a;
  color: #67c23a;
}
.record-body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}
.record-left {
  width: 360px;
  flex-shrink: 0;
}
.amount-area {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 12px 4px 18px;
  border-bottom: 1px dashed #ebeef5;
}
.amount-cur {
  font-size: 26px;
  color: #303133;
  font-weight: 600;
}
.amount-text {
  font-size: 42px;
  font-weight: 700;
  color: #303133;
  letter-spacing: 1px;
  font-variant-numeric: tabular-nums;
}
.keypad {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  padding-top: 14px;
}
.key {
  height: 52px;
  border-radius: 10px;
  border: 1px solid #dcdfe6;
  background: #fff;
  font-size: 20px;
  color: #303133;
  cursor: pointer;
  transition: background 0.15s;
}
.key:hover {
  background: #f5f7fa;
}
.key.back {
  font-size: 18px;
}
.record-right {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.block-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
}
.cats {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));
  gap: 10px;
}
.cat {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 10px 4px;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
}
.cat:hover {
  border-color: #409eff;
}
.cat.selected {
  border-color: #409eff;
  background: #ecf5ff;
}
.cat-name {
  font-size: 12px;
  color: #606266;
}
.cat-check {
  position: absolute;
  top: 4px;
  right: 4px;
  color: #409eff;
  font-size: 12px;
}
.info-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.info-label {
  width: 48px;
  font-size: 14px;
  color: #909399;
  text-align: right;
}
.save-area {
  display: flex;
  align-items: center;
  gap: 16px;
}
.save-btn {
  flex: 1;
  height: 48px;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #f56c6c, #f78989);
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(245, 108, 108, 0.3);
  transition: opacity 0.15s;
}
.save-btn:hover {
  opacity: 0.9;
}
.save-btn.income {
  background: linear-gradient(135deg, #67c23a, #85ce61);
  box-shadow: 0 4px 12px rgba(103, 194, 58, 0.3);
}
.save-btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}
</style>
