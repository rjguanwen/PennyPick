<template>
  <div class="recur-page">
    <div class="page-head">
      <span class="page-title">固定账单</span>
      <el-button type="primary" :icon="Plus" @click="openAdd">新增固定账单</el-button>
    </div>

    <!-- 记入月份 -->
    <div class="pp-card toolbar">
      <div class="tool-row">
        <span class="label">记入月份</span>
        <el-date-picker v-model="month" type="month" value-format="YYYY-MM" :clearable="false" style="width: 140px" />
        <el-button type="primary" :loading="applying" :disabled="!selectedIds.length" @click="applySelected">
          勾选项记入{{ monthLabel }}
        </el-button>
        <span class="tip">房贷、车贷等每月固定支出无需重复录入：勾选本月实际发生的项，一键记入账单。</span>
      </div>
    </div>

    <!-- 列表 -->
    <div class="pp-card list">
      <div class="check-all-row">
        <el-checkbox v-model="checkAll">全选启用项</el-checkbox>
        <span class="count-tip">已选 {{ selectedIds.length }} 项</span>
      </div>
      <div v-for="rb in list" :key="rb.id" class="rb-item">
        <el-checkbox :model-value="selectedIds.includes(rb.id)" @change="(v) => toggle(rb.id, v)" />
        <div class="rb-icon"><CatIcon :icon="catOf(rb.category_id)" :size="18" /></div>
        <div class="rb-info">
          <div class="rb-name">{{ rb.name }}<el-tag v-if="!rb.active" size="small" type="info" effect="plain" class="rb-inactive">停用</el-tag></div>
          <div class="rb-meta">每月{{ rb.day }}日 · {{ catName(rb.category_id) }}{{ accName(rb.account_id) }}</div>
        </div>
        <span class="rb-amount" :class="rb.type === 'income' ? 'inc' : 'exp'">¥{{ formatMoney(rb.amount) }}</span>
        <div class="rb-ops">
          <el-button link size="small" @click="openEdit(rb)">编辑</el-button>
          <el-button link type="danger" size="small" @click="remove(rb)">删除</el-button>
        </div>
      </div>
      <el-empty v-if="!list.length" description="还没有固定账单，点击右上角「新增固定账单」添加房贷、车贷等每月固定支出" :image-size="70" />
    </div>

    <!-- 对话框 -->
    <el-dialog v-model="dlgVisible" :title="dlgForm.id ? '编辑固定账单' : '新增固定账单'" width="440px">
      <el-form label-width="96px">
        <el-form-item label="名称">
          <el-input v-model="dlgForm.name" placeholder="如 房贷 / 车贷 / 物业费" maxlength="64" />
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="dlgForm.type" @change="dlgForm.category_id = null">
            <el-radio-button value="expense">支出</el-radio-button>
            <el-radio-button value="income">收入</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="dlgForm.category_id" placeholder="选择分类" style="width: 100%">
            <el-option v-for="cat in visibleCategories" :key="cat.id" :label="cat.name" :value="cat.id">
              <span style="margin-right: 4px"><el-icon style="vertical-align: middle"><component :is="cat.icon" /></el-icon></span>
              {{ cat.name }}
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="金额">
          <el-input-number v-model="dlgForm.amount" :min="0.01" :precision="2" style="width: 180px" />
        </el-form-item>
        <el-form-item label="账户">
          <el-select v-model="dlgForm.account_id" placeholder="选择账户（可选）" clearable style="width: 100%">
            <el-option v-for="a in accounts" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="每月日期">
          <el-input-number v-model="dlgForm.day" :min="1" :max="28" style="width: 140px" />
          <span class="tip-inline">每月{{ dlgForm.day }}日记入账单</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="dlgForm.note" placeholder="备注（可选）" maxlength="255" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="dlgForm.active" />
          <span class="tip-inline">停用的项默认不勾选</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDlg">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { accountApi, categoryApi, recurringBillApi } from '../api'
import { currentMonth, formatMoney } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'

const list = ref([])
const categories = ref([])
const accounts = ref([])
const month = ref(currentMonth())
const selectedIds = ref([])
const applying = ref(false)

const dlgVisible = ref(false)
const saving = ref(false)
const dlgForm = reactive({ id: null, name: '', type: 'expense', category_id: null, amount: null, account_id: null, day: 1, note: '', active: true })

const visibleCategories = computed(() => categories.value.filter((c) => c.type === dlgForm.type))
const monthLabel = computed(() => {
  const [y, m] = month.value.split('-')
  return `${y}年${Number(m)}月`
})
const checkAll = computed({
  get: () => list.value.length > 0 && selectedIds.value.length === list.value.length,
  set: (v) => {
    selectedIds.value = v ? list.value.map((x) => x.id) : []
  },
})

function catOf(id) {
  const c = categories.value.find((x) => x.id === id)
  return c ? c.icon : 'Money'
}
function catName(id) {
  const c = categories.value.find((x) => x.id === id)
  return c ? c.name : ''
}
function accName(id) {
  if (!id) return ''
  const a = accounts.value.find((x) => x.id === id)
  return a ? ` · ${a.name}` : ''
}

function toggle(id, v) {
  if (v) {
    if (!selectedIds.value.includes(id)) selectedIds.value.push(id)
  } else {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  }
}

async function load() {
  const [rbs, cats, accs] = await Promise.all([recurringBillApi.list(), categoryApi.list(), accountApi.list()])
  list.value = rbs || []
  categories.value = cats || []
  accounts.value = accs || []
  selectedIds.value = list.value.filter((x) => x.active).map((x) => x.id)
}

async function applySelected() {
  if (!selectedIds.value.length) {
    ElMessage.warning('请先勾选本月要记入的固定账单')
    return
  }
  applying.value = true
  try {
    const res = await recurringBillApi.apply({ month: month.value, ids: selectedIds.value })
    const dups = res.duplicated || []
    if (res.count === 0 && dups.length) {
      ElMessage.warning(`所选固定账单本月均已记过账，未重复记账：${dups.map((d) => d.name).join('、')}`)
    } else if (res.count > 0 && dups.length) {
      ElMessage.success(`已记入 ${res.count} 笔；以下本月已记过账，已跳过：${dups.map((d) => d.name).join('、')}`)
    } else {
      ElMessage.success(`已将 ${res.count} 笔固定账单记入 ${monthLabel.value}`)
    }
  } catch (e) {
    // 拦截器已提示
  } finally {
    applying.value = false
  }
}

function openAdd() {
  dlgForm.id = null
  dlgForm.name = ''
  dlgForm.type = 'expense'
  dlgForm.category_id = null
  dlgForm.amount = null
  dlgForm.account_id = null
  dlgForm.day = 1
  dlgForm.note = ''
  dlgForm.active = true
  dlgVisible.value = true
}

function openEdit(rb) {
  dlgForm.id = rb.id
  dlgForm.name = rb.name
  dlgForm.type = rb.type
  dlgForm.category_id = rb.category_id
  dlgForm.amount = rb.amount
  dlgForm.account_id = rb.account_id || null
  dlgForm.day = rb.day || 1
  dlgForm.note = rb.note || ''
  dlgForm.active = !!rb.active
  dlgVisible.value = true
}

async function saveDlg() {
  const payload = {
    name: dlgForm.name.trim(),
    type: dlgForm.type,
    category_id: dlgForm.category_id,
    account_id: dlgForm.account_id || null,
    amount: dlgForm.amount,
    day: dlgForm.day,
    note: dlgForm.note,
    active: dlgForm.active,
  }
  if (!payload.name) {
    ElMessage.warning('请输入名称')
    return
  }
  if (!payload.category_id) {
    ElMessage.warning('请选择分类')
    return
  }
  if (!payload.amount || payload.amount <= 0) {
    ElMessage.warning('请输入金额')
    return
  }
  saving.value = true
  try {
    if (dlgForm.id) {
      await recurringBillApi.update(dlgForm.id, payload)
      ElMessage.success('已保存')
    } else {
      await recurringBillApi.create(payload)
      ElMessage.success('已添加')
    }
    dlgVisible.value = false
    load()
  } catch (e) {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

async function remove(rb) {
  try {
    await ElMessageBox.confirm(`确定删除固定账单「${rb.name}」吗？`, '删除', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await recurringBillApi.remove(rb.id)
    ElMessage.success('已删除')
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
  margin-bottom: 14px;
}
.page-title {
  font-size: 17px;
  font-weight: 700;
}
.toolbar {
  margin-bottom: 12px;
}
.tool-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.label {
  font-size: 14px;
  color: #606266;
}
.tip {
  font-size: 12px;
  color: #909399;
}
.list {
  padding: 10px 18px;
}
.check-all-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0 8px;
  border-bottom: 1px solid #f2f3f5;
  margin-bottom: 4px;
}
.count-tip {
  font-size: 12px;
  color: #909399;
}
.rb-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 0;
  border-bottom: 1px solid #f2f3f5;
}
.rb-item:last-child {
  border-bottom: none;
}
.rb-icon {
  flex-shrink: 0;
}
.rb-info {
  flex: 1;
  min-width: 0;
}
.rb-name {
  font-size: 15px;
  font-weight: 600;
}
.rb-inactive {
  margin-left: 8px;
}
.rb-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
.rb-amount {
  font-weight: 700;
  font-size: 15px;
}
.rb-amount.exp {
  color: #f56c6c;
}
.rb-amount.inc {
  color: #67c23a;
}
.rb-ops {
  flex-shrink: 0;
}
.tip-inline {
  font-size: 12px;
  color: #c0c4cc;
  margin-left: 10px;
}
</style>
