<template>
  <div class="batch-page">
    <div class="page-head">
      <span class="page-title">批量记账</span>
      <span class="page-sub">几天没记账？选一个账户，逐行补录，一次保存多笔。</span>
    </div>

    <!-- 顶部：账户 + 类型 -->
    <div class="pp-card toolbar">
      <div class="tool-row">
        <span class="label">账户</span>
        <el-select v-model="account_id" placeholder="选择账户" style="width: 200px">
          <el-option v-for="a in accounts" :key="a.id" :label="a.name" :value="a.id">
            <span style="margin-right: 4px"><el-icon style="vertical-align: middle"><component :is="a.icon || 'Wallet'" /></el-icon></span>
            {{ a.name }}
          </el-option>
        </el-select>
        <span class="label sep">类型</span>
        <el-radio-group v-model="billType" size="small" @change="onTypeChange">
          <el-radio-button value="expense">支出</el-radio-button>
          <el-radio-button value="income">收入</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <!-- 行列表 -->
    <div class="pp-card rows-card">
      <div class="row-head">
        <span class="col-cat">分类</span>
        <span class="col-amt">金额</span>
        <span class="col-date">日期</span>
        <span class="col-tag">标签</span>
        <span class="col-note">备注</span>
        <span class="col-op"></span>
      </div>
      <div v-for="(row, idx) in rows" :key="idx" class="row">
        <el-select v-model="row.category_id" placeholder="分类" class="col-cat" filterable>
          <el-option v-for="cat in visibleCategories" :key="cat.id" :label="cat.name" :value="cat.id">
            <span style="margin-right: 4px"><el-icon style="vertical-align: middle"><component :is="cat.icon" /></el-icon></span>
            {{ cat.name }}
          </el-option>
        </el-select>
        <el-input-number v-model="row.amount" :min="0.01" :precision="2" :step="5" controls-position="right" placeholder="金额" class="col-amt" />
        <el-date-picker v-model="row.occurred_at" type="date" value-format="YYYY-MM-DD" placeholder="日期" class="col-date" :clearable="false" />
        <el-select
          v-model="row.tag_ids"
          multiple
          filterable
          allow-create
          default-first-option
          :limit="maxBillTags"
          collapse-tags
          collapse-tags-tooltip
          placeholder="标签（可选）"
          class="col-tag"
        >
          <el-option v-for="t in tags" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <el-input v-model="row.note" placeholder="备注（可选）" maxlength="100" class="col-note" @keyup.enter="saveAll" />
        <el-button link type="danger" :icon="Delete" class="row-del" @click="removeRow(idx)" />
      </div>
      <el-empty v-if="!rows.length" description="点击下方「添加一行」开始录入" :image-size="60" />
      <div class="row-add">
        <el-button :icon="Plus" @click="addRow">添加一行</el-button>
      </div>
    </div>

    <!-- 底部：合计 + 保存 -->
    <div class="pp-card footer">
      <div class="total">共 <b>{{ validCount }}</b> 笔，合计 <b class="total-amt">¥{{ formatMoney(totalAmount) }}</b></div>
      <el-button type="primary" size="large" :loading="saving" :disabled="!canSave" @click="saveAll">保存全部</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import { accountApi, billApi, categoryApi, tagApi } from '../api'
import { formatMoney, nowDate } from '../utils/format'

const maxBillTags = 8

const accounts = ref([])
const categories = ref([])
const tags = ref([])
const account_id = ref(null)
const billType = ref('expense')
const rows = ref([])
const saving = ref(false)

const visibleCategories = computed(() => categories.value.filter((c) => c.type === billType.value))

const validRows = computed(() => rows.value.filter((r) => r.category_id && r.amount && r.amount > 0))
const validCount = computed(() => validRows.value.length)
const totalAmount = computed(() => validRows.value.reduce((s, r) => s + Number(r.amount || 0), 0))
const canSave = computed(() => account_id.value && validCount.value > 0)

function addRow() {
  const first = visibleCategories.value[0]
  rows.value.push({
    category_id: first ? first.id : null,
    amount: null,
    occurred_at: nowDate(),
    note: '',
    tag_ids: [],
  })
}

function removeRow(idx) {
  rows.value.splice(idx, 1)
}

function onTypeChange() {
  // 切换类型后清空行的分类选择，避免与新类型不匹配
  rows.value.forEach((r) => {
    r.category_id = null
  })
}

// 把各行 tag_ids 中的字符串（新建标签名）转为真实标签 id
async function resolveTags() {
  const newNames = []
  for (const r of validRows.value) {
    for (const v of r.tag_ids || []) {
      if (typeof v === 'string') {
        const n = v.trim()
        if (n && !newNames.includes(n)) newNames.push(n)
      }
    }
  }
  const nameToId = {}
  for (const n of newNames) {
    const t = await tagApi.create({ name: n })
    nameToId[n] = t.id
  }
  return validRows.value.map((r) => {
    const ids = []
    for (const v of r.tag_ids || []) {
      if (typeof v === 'number') {
        ids.push(v)
      } else if (typeof v === 'string' && nameToId[v.trim()]) {
        ids.push(nameToId[v.trim()])
      }
    }
    return ids
  })
}

async function saveAll() {
  if (!account_id.value) {
    ElMessage.warning('请先选择账户')
    return
  }
  if (validCount.value === 0) {
    ElMessage.warning('请至少录入一笔（分类 + 金额）')
    return
  }
  saving.value = true
  try {
    const tagIdLists = await resolveTags()
    const items = validRows.value.map((r, i) => ({
      category_id: r.category_id,
      type: billType.value,
      amount: r.amount,
      note: r.note || '',
      occurred_at: r.occurred_at,
      tag_ids: tagIdLists[i] || [],
    }))
    const res = await billApi.createBatch({ account_id: account_id.value, items })
    ElMessage.success(`已保存 ${res.count || items.length} 笔账单`)
    rows.value = []
    addRow()
  } catch (e) {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  const [accs, cats, tgs] = await Promise.all([accountApi.list(), categoryApi.list(), tagApi.list()])
  accounts.value = accs || []
  categories.value = cats || []
  tags.value = tgs || []
  addRow()
})
</script>

<style scoped>
.batch-page {
  max-width: 1000px;
  margin: 0 auto;
}
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
.toolbar {
  margin-bottom: 12px;
}
.tool-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.label {
  font-size: 14px;
  color: #606266;
}
.label.sep {
  margin-left: 12px;
}
.rows-card {
  margin-bottom: 12px;
}
.row-head,
.row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.row-head {
  font-size: 12px;
  color: #909399;
  padding: 4px 0 8px;
}
.row {
  padding: 8px 0;
  border-top: 1px solid #f5f7fa;
}
.col-cat {
  width: 170px;
}
.col-amt {
  width: 140px;
}
.col-date {
  width: 150px;
}
.col-tag {
  flex: 1 1 200px;
  min-width: 200px;
}
.col-note {
  flex: 1 1 160px;
  min-width: 160px;
}
.col-op {
  width: 40px;
}
.row-del {
  flex-shrink: 0;
}
.row-add {
  margin-top: 10px;
  border-top: 1px dashed #e4e7ed;
  padding-top: 10px;
}
.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.total {
  font-size: 14px;
  color: #606266;
}
.total b {
  color: #303133;
}
.total-amt {
  color: #f56c6c;
  font-size: 18px;
  margin-left: 4px;
}
</style>
