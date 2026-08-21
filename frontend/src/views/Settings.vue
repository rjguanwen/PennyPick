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
          <CatIcon :icon="acc.icon" color="#409eff" :size="16" />
          <span class="acc-name">{{ acc.name }}</span>
          <el-tag v-if="acc.is_credit" size="small" type="warning" effect="plain">先用后还</el-tag>
          <el-button link size="small" @click="openEdit(acc)">编辑</el-button>
          <el-button link type="danger" size="small" @click="removeAccount(acc)">删除</el-button>
        </div>
        <el-empty v-if="!accounts.length" description="暂无账户" :image-size="50" />
      </div>
      <div class="add-acc">
        <el-button type="primary" :icon="Plus" @click="openAdd">新建账户</el-button>
        <span class="acc-tip">先用后还账户可在「还款」页按月标记还款</span>
      </div>
    </div>

    <!-- 账户对话框 -->
    <el-dialog v-model="accVisible" :title="accForm.id ? '编辑账户' : '新建账户'" width="430px">
      <el-form label-width="92px">
        <el-form-item label="名称">
          <el-input v-model="accForm.name" placeholder="如 信用卡A / 花呗" maxlength="32" />
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
        <el-form-item v-if="accForm.is_credit" label="每月还款日">
          <el-input-number v-model="accForm.repay_day" :min="1" :max="28" />
          <span class="acc-tip-inline">每月{{ accForm.repay_day }}日前还款</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="accVisible = false">取消</el-button>
        <el-button type="primary" :loading="accSaving" @click="saveAccount">保存</el-button>
      </template>
    </el-dialog>

    <!-- 标签管理 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><CollectionTag /></el-icon> 标签管理</div>
      <div class="tag-list">
        <div v-for="t in tags" :key="t.id" class="tag-item">
          <el-tag type="info" effect="plain" class="tag-chip">{{ t.name }}</el-tag>
          <el-button link type="danger" size="small" @click="removeTag(t)">删除</el-button>
        </div>
        <el-empty v-if="!tags.length" description="暂无标签" :image-size="50" />
      </div>
      <div class="add-tag">
        <el-input v-model="newTag" placeholder="新标签名称，如 出差、宝宝" style="flex: 1" maxlength="16" @keyup.enter="addTag" />
        <el-button type="primary" :icon="Plus" @click="addTag">添加</el-button>
      </div>
      <div class="tag-tip">每条账单最多打 8 个标签；记账时可输入新标签名直接创建。</div>
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
import { CollectionTag, Download, Plus, Lock, Wallet } from '@element-plus/icons-vue'
import * as ElementPlusIcons from '@element-plus/icons-vue'
import { accountApi, authApi, exportApi, tagApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { nowDate } from '../utils/format'
import CatIcon from '../components/CatIcon.vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const auth = useAuthStore()

const avatarText = computed(() => (auth.user?.nickname || auth.user?.username || '?').charAt(0))

const accounts = ref([])
const accVisible = ref(false)
const accSaving = ref(false)
const iconPicks = [
  'Money', 'Wallet', 'CreditCard', 'BankCard', 'ChatDotRound', 'Coin',
  'PiggyBank', 'Present', 'House', 'ShoppingBag', 'Cellphone', 'More',
]
const iconOptions = iconPicks.filter((n) => n in ElementPlusIcons)
const accForm = reactive({ id: null, name: '', icon: 'Wallet', is_credit: false, repay_day: 25 })
const tags = ref([])
const newTag = ref('')

const exportRange = ref([])
const exportType = ref('')
const exportFormat = ref('csv')
const exporting = ref(false)

const pwd = reactive({ old_password: '', new_password: '', confirm: '' })
const pwdSaving = ref(false)

async function loadAccounts() {
  accounts.value = (await accountApi.list()) || []
}

async function loadTags() {
  tags.value = (await tagApi.list()) || []
}

async function addTag() {
  const name = newTag.value.trim()
  if (!name) {
    ElMessage.warning('请输入标签名称')
    return
  }
  await tagApi.create({ name })
  ElMessage.success('已添加')
  newTag.value = ''
  loadTags()
}

async function removeTag(tag) {
  try {
    await ElMessageBox.confirm(`确定删除标签「${tag.name}」吗？`, '删除标签', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await tagApi.remove(tag.id)
    ElMessage.success('已删除')
    loadTags()
  } catch (e) {
    // 拦截器已提示
  }
}

function openAdd() {
  accForm.id = null
  accForm.name = ''
  accForm.icon = 'Wallet'
  accForm.is_credit = false
  accForm.repay_day = 25
  accVisible.value = true
}

function openEdit(a) {
  accForm.id = a.id
  accForm.name = a.name
  accForm.icon = a.icon || 'Wallet'
  accForm.is_credit = !!a.is_credit
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
      repay_day: accForm.repay_day,
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
    router.push('/login')
  } finally {
    pwdSaving.value = false
  }
}

onMounted(async () => {
  loadAccounts()
  loadTags()
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
  align-items: center;
  gap: 10px;
}
.acc-tip {
  font-size: 12px;
  color: #c0c4cc;
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
.tag-list {
  margin-bottom: 12px;
}
.tag-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid #f2f3f5;
}
.tag-item:last-child {
  border-bottom: none;
}
.tag-chip {
  flex-shrink: 0;
}
.add-tag {
  display: flex;
  gap: 8px;
}
.tag-tip {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 8px;
}
</style>
