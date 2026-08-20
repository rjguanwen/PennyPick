<template>
  <div class="settings">
    <!-- 个人信息 -->
    <div class="pp-card profile">
      <el-avatar :size="52" class="avatar">{{ avatarText }}</el-avatar>
      <div class="profile-info">
        <div class="name">{{ auth.user?.nickname || auth.user?.username }}</div>
        <div class="username">@{{ auth.user?.username }} · 拾财用户</div>
      </div>
    </div>

    <!-- 导出账单 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><Download /></el-icon> 导出账单</div>
      <div class="export-form">
        <div class="field">
          <span class="label">时间范围</span>
          <el-date-picker
            v-model="exportRange"
            type="daterange"
            value-format="YYYY-MM-DD"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="width: 100%"
          />
        </div>
        <div class="field">
          <span class="label">账单类型</span>
          <el-select v-model="exportType" style="width: 100%">
            <el-option label="全部" value="" />
            <el-option label="仅支出" value="expense" />
            <el-option label="仅收入" value="income" />
          </el-select>
        </div>
        <div class="field">
          <span class="label">导出格式</span>
          <el-select v-model="exportFormat" style="width: 100%">
            <el-option label="CSV（Excel 可直接打开）" value="csv" />
          </el-select>
        </div>
        <el-button type="primary" :icon="Download" :loading="exporting" @click="doExport">
          导出 CSV
        </el-button>
      </div>
    </div>

    <!-- 账户管理 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><Wallet /></el-icon> 账户管理</div>
      <div class="acc-list">
        <div v-for="acc in accounts" :key="acc.id" class="acc-item">
          <CatIcon icon="Wallet" color="#409eff" :size="16" />
          <span class="acc-name">{{ acc.name }}</span>
          <el-button link type="danger" size="small" @click="removeAccount(acc)">删除</el-button>
        </div>
      </div>
      <div class="add-acc">
        <el-input v-model="newAccount" placeholder="新账户名称，如 零钱" style="flex: 1" maxlength="32" @keyup.enter="addAccount" />
        <el-button type="primary" :icon="Plus" @click="addAccount">添加</el-button>
      </div>
    </div>

    <!-- 修改密码 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><Lock /></el-icon> 修改密码</div>
      <el-form label-position="top">
        <el-form-item label="原密码">
          <el-input v-model="pwd.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwd.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="pwd.confirm" type="password" show-password />
        </el-form-item>
        <el-button type="primary" :icon="Lock" :loading="pwdSaving" @click="changePassword">保存密码</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Download, Plus, Lock, Wallet } from '@element-plus/icons-vue'
import { accountApi, authApi, exportApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { nowDate } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'

const auth = useAuthStore()

const avatarText = computed(() => (auth.user?.nickname || auth.user?.username || '?').charAt(0))

const accounts = ref([])
const newAccount = ref('')

const exportRange = ref([])
const exportType = ref('')
const exportFormat = ref('csv')
const exporting = ref(false)

const pwd = reactive({ old_password: '', new_password: '', confirm: '' })
const pwdSaving = ref(false)

async function loadAccounts() {
  accounts.value = (await accountApi.list()) || []
}

async function addAccount() {
  const name = newAccount.value.trim()
  if (!name) {
    ElMessage.warning('请输入账户名称')
    return
  }
  await accountApi.create({ name, icon: 'Wallet' })
  ElMessage.success('已添加')
  newAccount.value = ''
  loadAccounts()
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

async function doExport() {
  const params = {
    start: exportRange.value?.[0] || undefined,
    end: exportRange.value?.[1] || undefined,
    type: exportType.value || undefined,
  }
  exporting.value = true
  try {
    const blob = await exportApi.download(params)
    const filename = `pennypick_bills_${exportRange.value?.[0] || 'all'}_${exportRange.value?.[1] || 'now'}.csv`
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e) {
    ElMessage.error('导出失败')
  } finally {
    exporting.value = false
  }
}

async function changePassword() {
  if (!pwd.old_password || !pwd.new_password) {
    ElMessage.warning('请填写原密码和新密码')
    return
  }
  if (pwd.new_password.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (pwd.new_password !== pwd.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  pwdSaving.value = true
  try {
    await authApi.changePassword({ old_password: pwd.old_password, new_password: pwd.new_password })
    ElMessage.success('密码已修改，请重新登录')
    auth.logout()
    location.href = '/login'
  } finally {
    pwdSaving.value = false
  }
}

onMounted(async () => {
  loadAccounts()
  const end = new Date()
  const start = new Date()
  start.setMonth(start.getMonth() - 1)
  exportRange.value = [nowDate(start), nowDate(end)]
})
</script>

<style scoped>
.settings {
  max-width: 640px;
  margin: 0 auto;
}
.profile {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 12px;
}
.avatar {
  background: #409eff;
  font-size: 22px;
}
.profile-info .name {
  font-size: 17px;
  font-weight: 700;
}
.profile-info .username {
  font-size: 13px;
  color: #909399;
  margin-top: 2px;
}
.pp-card {
  margin-bottom: 12px;
}
.card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  margin-bottom: 14px;
}
.export-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.field .label {
  display: block;
  font-size: 13px;
  color: #909399;
  margin-bottom: 6px;
}
.acc-list {
  margin-bottom: 12px;
}
.acc-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid #f2f3f5;
}
.acc-name {
  flex: 1;
  font-size: 15px;
}
.add-acc {
  display: flex;
  gap: 8px;
}
</style>
