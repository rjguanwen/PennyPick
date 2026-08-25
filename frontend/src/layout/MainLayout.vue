<template>
  <el-container class="layout">
    <!-- 侧边栏 -->
    <el-aside width="220px" class="aside">
      <div class="logo">
        <span class="logo-icon">💰</span>
        <span class="logo-text">拾财</span>
        <span class="logo-sub">PennyPick</span>
      </div>
      <el-menu
        :default-active="$route.path"
        router
        background-color="#001529"
        text-color="rgba(255,255,255,0.68)"
        active-text-color="#ffffff"
      >
        <el-menu-item index="/dashboard"><el-icon><HomeFilled /></el-icon><span>首页</span></el-menu-item>
        <el-menu-item index="/record"><el-icon><CirclePlusFilled /></el-icon><span>记一笔</span></el-menu-item>
        <el-menu-item index="/batch-record"><el-icon><EditPen /></el-icon><span>批量记账</span></el-menu-item>
        <el-menu-item index="/recurring-bills"><el-icon><Calendar /></el-icon><span>固定账单</span></el-menu-item>
        <el-menu-item index="/bills"><el-icon><Tickets /></el-icon><span>账单</span></el-menu-item>
        <el-menu-item index="/repayment"><el-icon><CreditCard /></el-icon><span>还款</span></el-menu-item>
        <el-menu-item index="/stats"><el-icon><DataAnalysis /></el-icon><span>统计</span></el-menu-item>
        <el-menu-item index="/budget"><el-icon><Odometer /></el-icon><span>预算</span></el-menu-item>
        <el-menu-item index="/categories"><el-icon><Grid /></el-icon><span>分类管理</span></el-menu-item>
        <el-menu-item index="/accounts"><el-icon><Wallet /></el-icon><span>账户管理</span></el-menu-item>
        <el-menu-item index="/settings"><el-icon><Setting /></el-icon><span>设置</span></el-menu-item>
      </el-menu>
      <div class="aside-footer">
        <el-dropdown @command="onCommand" trigger="click">
          <span class="user-info">
            <el-avatar :size="30" class="avatar">{{ avatarText }}</el-avatar>
            <span class="name">{{ auth.displayName }}</span>
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="page-title">{{ pageTitle }}</div>
        <el-dropdown @command="onCommand" trigger="click">
          <span class="user-info">
            <el-avatar :size="30" class="avatar">{{ avatarText }}</el-avatar>
            <span class="name">{{ auth.displayName }}</span>
            <el-tag size="small" type="success">拾财用户</el-tag>
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="settings">设置</el-dropdown-item>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const pageTitle = computed(() => route.meta.title || '')
const avatarText = computed(() => (auth.displayName || '?').charAt(0))

function onCommand(cmd) {
  if (cmd === 'logout') {
    ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' })
      .then(() => {
        auth.logout()
        router.push('/login')
      })
      .catch(() => {})
  } else if (cmd === 'settings') {
    router.push('/settings')
  }
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background: #001529;
  display: flex;
  flex-direction: column;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #fff;
}
.logo-icon {
  font-size: 22px;
}
.logo-text {
  font-size: 18px;
  font-weight: 700;
}
.logo-sub {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.45);
  letter-spacing: 1px;
}
.el-menu {
  border-right: none;
  flex: 1;
}
.aside-footer {
  padding: 12px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  outline: none;
  color: #303133;
}
.aside-footer .user-info {
  color: #fff;
}
.aside-footer .name {
  color: rgba(255, 255, 255, 0.85);
}
.user-info .name {
  font-size: 14px;
}
.avatar {
  background: #409eff;
  flex-shrink: 0;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #eee;
}
.page-title {
  font-size: 16px;
  font-weight: 600;
}
.main {
  background: #f5f7fa;
  overflow: auto;
  padding: 20px;
}
</style>
