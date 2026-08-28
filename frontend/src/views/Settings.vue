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

    <!-- 关于 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><InfoFilled /></el-icon> 关于</div>
      <div class="about">
        <div class="about-logo">💰</div>
        <div class="about-name">拾财 PennyPick</div>
        <div class="about-desc">个人记账应用：轻松记下每一笔消费，多维度统计分析，科学规划预算，帮你管好每一分钱。</div>
        <div class="about-item"><span>版本</span><b>1.1.0</b></div>
        <div class="about-item"><span>开发者</span><b>关文</b></div>
        <div class="about-item"><span>邮箱</span><b>rjguanwen001@163.com</b></div>
        <div class="about-item"><span>发布时间</span><b>2026-08-28</b></div>
        <div class="about-item"><span>技术栈</span><b>Vue 3 · Element Plus · Go</b></div>
      </div>
    </div>

    <!-- 赞助支持 -->
    <div class="pp-card">
      <div class="card-title"><el-icon><Coffee /></el-icon> 赞助支持</div>
      <div class="donate">
        <p class="donate-tip">拾财完全免费使用。如果您觉得它好用，欢迎自愿扫码请开发者喝杯咖啡——付多少、付不付，都不影响任何功能的使用。</p>
        <img :src="donateImg" class="donate-qr" alt="微信收款码" />
        <p class="donate-note">微信扫码支付</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CollectionTag, Download, Plus, Lock, InfoFilled, Coffee } from '@element-plus/icons-vue'
import donateImg from '../assets/skzh.png'
import { authApi, exportApi, tagApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { nowDate } from '../utils/format'
import { useRouter } from 'vue-router'

const router = useRouter()
const auth = useAuthStore()

const avatarText = computed(() => (auth.user?.nickname || auth.user?.username || '?').charAt(0))

const tags = ref([])
const newTag = ref('')

const exportRange = ref([])
const exportType = ref('')
const exportFormat = ref('csv')
const exporting = ref(false)

const pwd = reactive({ old_password: '', new_password: '', confirm: '' })
const pwdSaving = ref(false)

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
.about {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}
.about-logo {
  font-size: 44px;
  margin-bottom: 6px;
}
.about-name {
  font-size: 18px;
  font-weight: 700;
  color: #303133;
}
.about-desc {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
  margin: 8px 0 14px;
  max-width: 420px;
}
.about-item {
  display: flex;
  justify-content: space-between;
  width: 100%;
  max-width: 340px;
  padding: 6px 0;
  font-size: 14px;
  border-bottom: 1px solid #f5f7fa;
}
.about-item:last-child {
  border-bottom: none;
}
.about-item span {
  color: #909399;
}
.about-item b {
  color: #303133;
}
.donate {
  text-align: center;
}
.donate-tip {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
  margin: 0 0 14px;
  text-align: left;
}
.donate-qr {
  width: 200px;
  height: 200px;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(31, 45, 61, 0.1);
}
.donate-note {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 8px;
}
</style>
