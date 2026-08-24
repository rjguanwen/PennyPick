<template>
  <div class="account-page">
    <div class="page-head">
      <span class="page-title">账户管理</span>
      <el-button type="primary" :icon="Plus" @click="openAdd">新建账户</el-button>
    </div>

    <el-alert type="info" :closable="false" show-icon class="tip"
      title="先用后还账户（如信用卡、花呗）可设置每月出账日与还款日"
      description="还款页按「上月出账日 ~ 本月出账日」的账期统计本期应还金额，超过还款日未标记时会提醒。"
    />

    <div class="list">
      <div v-for="acc in accounts" :key="acc.id" class="acc-item">
        <div class="acc-icon"><el-icon :size="20"><component :is="acc.icon || 'Wallet'" /></el-icon></div>
        <div class="acc-info">
          <div class="acc-name">{{ acc.name }}</div>
          <div v-if="acc.is_credit" class="acc-meta">出账日每月{{ acc.statement_day || 1 }}日 · 还款日每月{{ acc.repay_day || 25 }}日</div>
          <div v-else class="acc-meta">普通账户</div>
        </div>
        <el-tag v-if="acc.is_credit" size="small" type="warning" effect="plain" class="acc-tag">先用后还</el-tag>
        <div class="acc-ops">
          <el-button link size="small" @click="openEdit(acc)">编辑</el-button>
          <el-button link type="danger" size="small" @click="removeAccount(acc)">删除</el-button>
        </div>
      </div>
      <el-empty v-if="!accounts.length" description="暂无账户" />
    </div>

    <!-- 账户对话框 -->
    <el-dialog v-model="accVisible" :title="accForm.id ? '编辑账户' : '新建账户'" width="440px">
      <el-form label-width="96px">
        <el-form-item label="名称">
          <el-input v-model="accForm.name" placeholder="如 信用卡A / 花呗 / 现金" maxlength="32" />
        </el-form-item>
        <el-form-item label="图标">
          <div class="icon-grid">
            <button
              v-for="ic in iconOptions"
              :key="ic"
              type="button"
              class="icon-opt pp-tap"
              :class="{ selected: accForm.icon === ic }"
              @click="accForm.icon = ic"
            >
              <el-icon :size="20"><component :is="ic" /></el-icon>
            </button>
          </div>
        </el-form-item>
        <el-form-item label="先用后还">
          <el-switch v-model="accForm.is_credit" />
          <span class="acc-tip-inline">先用后还的账户按月还款</span>
        </el-form-item>
        <template v-if="accForm.is_credit">
          <el-form-item label="每月出账日">
            <el-input-number v-model="accForm.statement_day" :min="1" :max="28" />
            <span class="acc-tip-inline">每月{{ accForm.statement_day }}日生成账单</span>
          </el-form-item>
          <el-form-item label="每月还款日">
            <el-input-number v-model="accForm.repay_day" :min="1" :max="28" />
            <span class="acc-tip-inline">每月{{ accForm.repay_day }}日前还款</span>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="accVisible = false">取消</el-button>
        <el-button type="primary" :loading="accSaving" @click="saveAccount">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import * as ElementPlusIcons from '@element-plus/icons-vue'
import { accountApi } from '../api'

const accounts = ref([])
const accVisible = ref(false)
const accSaving = ref(false)
const iconPicks = [
  'Money', 'Wallet', 'CreditCard', 'BankCard', 'ChatDotRound', 'Coin',
  'PiggyBank', 'Present', 'House', 'ShoppingBag', 'Cellphone', 'More',
]
const iconOptions = iconPicks.filter((n) => n in ElementPlusIcons)
const accForm = reactive({ id: null, name: '', icon: 'Wallet', is_credit: false, statement_day: 1, repay_day: 25 })

async function loadAccounts() {
  accounts.value = (await accountApi.list()) || []
}

function openAdd() {
  accForm.id = null
  accForm.name = ''
  accForm.icon = 'Wallet'
  accForm.is_credit = false
  accForm.statement_day = 1
  accForm.repay_day = 25
  accVisible.value = true
}

function openEdit(a) {
  accForm.id = a.id
  accForm.name = a.name
  accForm.icon = a.icon || 'Wallet'
  accForm.is_credit = !!a.is_credit
  accForm.statement_day = a.statement_day || 1
  accForm.repay_day = a.repay_day || 25
  accVisible.value = true
}

async function saveAccount() {
  const name = accForm.name.trim()
  if (!name) {
    ElMessage.warning('请输入账户名称')
    return
  }
  accSaving.value = true
  try {
    const payload = {
      name,
      icon: accForm.icon,
      is_credit: accForm.is_credit,
      statement_day: accForm.is_credit ? accForm.statement_day : 0,
      repay_day: accForm.is_credit ? accForm.repay_day : 0,
    }
    if (accForm.id) {
      await accountApi.update(accForm.id, payload)
      ElMessage.success('已保存')
    } else {
      await accountApi.create(payload)
      ElMessage.success('已添加')
    }
    accVisible.value = false
    loadAccounts()
  } catch (e) {
    // 拦截器已提示
  } finally {
    accSaving.value = false
  }
}

async function removeAccount(acc) {
  try {
    await ElMessageBox.confirm(`确定删除账户「${acc.name}」吗？`, '删除账户', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await accountApi.remove(acc.id)
    ElMessage.success('已删除')
    loadAccounts()
  } catch (e) {
    // 拦截器已提示
  }
}

onMounted(loadAccounts)
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
.tip {
  margin-bottom: 14px;
}
.list {
  background: #fff;
  border-radius: 10px;
  padding: 4px 18px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}
.acc-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 0;
  border-bottom: 1px solid #f2f3f5;
}
.acc-item:last-child {
  border-bottom: none;
}
.acc-icon {
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  border-radius: 10px;
  color: #409eff;
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
.acc-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
.acc-tag {
  flex-shrink: 0;
}
.acc-ops {
  flex-shrink: 0;
}
.icon-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.icon-opt {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dcdfe6;
  background: #fff;
  border-radius: 8px;
  cursor: pointer;
  color: #606266;
  transition: all 0.15s;
}
.icon-opt.selected {
  border-color: #409eff;
  color: #409eff;
  background: #ecf5ff;
}
.acc-tip-inline {
  font-size: 12px;
  color: #c0c4cc;
  margin-left: 10px;
}
</style>
