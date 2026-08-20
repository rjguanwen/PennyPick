<template>
  <el-dialog
    :model-value="modelValue"
    :title="bill ? '编辑账单' : '记一笔'"
    width="min(420px, 92vw)"
    :close-on-click-modal="false"
    @update:model-value="(v) => emit('update:modelValue', v)"
    @open="onOpen"
  >
    <el-form label-position="top">
      <el-radio-group v-model="form.type" class="type-switch" size="large">
        <el-radio-button value="expense">支出</el-radio-button>
        <el-radio-button value="income">收入</el-radio-button>
      </el-radio-group>

      <el-form-item label="金额">
        <el-input-number
          v-model="form.amount"
          :min="0.01"
          :max="999999999"
          :precision="2"
          :step="1"
          controls-position="right"
          style="width: 100%"
          placeholder="请输入金额"
        />
      </el-form-item>

      <el-form-item label="分类">
        <el-select v-model="form.category_id" placeholder="请选择分类" style="width: 100%">
          <el-option
            v-for="cat in visibleCategories"
            :key="cat.id"
            :label="cat.name"
            :value="cat.id"
          >
            <span class="opt"><el-icon><component :is="cat.icon" /></el-icon>{{ cat.name }}</span>
          </el-option>
        </el-select>
      </el-form-item>

      <el-form-item label="日期">
        <el-date-picker
          v-model="form.occurred_at"
          type="datetime"
          value-format="YYYY-MM-DD HH:mm"
          placeholder="选择日期时间"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item label="账户">
        <el-select v-model="form.account_id" placeholder="选择账户" clearable style="width: 100%">
          <el-option v-for="acc in accounts" :key="acc.id" :label="acc.name" :value="acc.id" />
        </el-select>
      </el-form-item>

      <el-form-item label="备注">
        <el-input v-model="form.note" maxlength="255" placeholder="备注（选填）" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { billApi } from '../api'
import { nowDateTime } from '../utils/format'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  bill: { type: Object, default: null }, // 编辑对象
  categories: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue', 'saved'])

const saving = ref(false)
const form = reactive({
  type: 'expense',
  amount: null,
  category_id: null,
  account_id: null,
  occurred_at: nowDateTime(),
  note: '',
})

const visibleCategories = computed(() =>
  props.categories.filter((c) => c.type === form.type),
)

// 切换类型时重置分类选择
watch(
  () => form.type,
  () => {
    form.category_id = null
  },
)

function onOpen() {
  if (props.bill) {
    form.type = props.bill.type || 'expense'
    form.amount = Number(props.bill.amount)
    form.category_id = props.bill.category_id
    form.account_id = props.bill.account_id || null
    form.occurred_at = (props.bill.occurred_at || nowDateTime()).slice(0, 16)
    form.note = props.bill.note || ''
  } else {
    form.type = 'expense'
    form.amount = null
    form.category_id = null
    form.account_id = null
    form.occurred_at = nowDateTime()
    form.note = ''
  }
}

async function save() {
  if (!form.amount || form.amount <= 0) {
    ElMessage.warning('请输入金额')
    return
  }
  if (!form.category_id) {
    ElMessage.warning('请选择分类')
    return
  }
  saving.value = true
  try {
    const payload = {
      type: form.type,
      amount: form.amount,
      category_id: form.category_id,
      account_id: form.account_id,
      occurred_at: form.occurred_at,
      note: form.note,
    }
    if (props.bill) {
      await billApi.update(props.bill.id, payload)
    } else {
      await billApi.create(payload)
    }
    ElMessage.success(props.bill ? '修改成功' : '记账成功')
    emit('update:modelValue', false)
    emit('saved')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.type-switch {
  margin-bottom: 16px;
}
.opt {
  display: flex;
  align-items: center;
  gap: 6px;
}
</style>
