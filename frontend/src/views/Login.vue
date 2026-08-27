<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <div class="brand-icon">💰</div>
        <h1>拾财</h1>
        <p>PennyPick · 轻松记下每一笔</p>
      </div>

      <!-- 自定义 Tab 切换（不使用 el-tabs，规避组件卸载时的 parentNode 崩溃） -->
      <div class="tab-switch">
        <button class="tab-btn pp-tap" :class="{ active: tab === 'login' }" type="button" @click="tab = 'login'">登录</button>
        <button class="tab-btn pp-tap" :class="{ active: tab === 'register' }" type="button" @click="tab = 'register'">注册</button>
      </div>

      <el-form v-if="tab === 'login'" @submit.prevent="onLogin">
        <el-form-item>
          <el-input v-model="loginForm.username" placeholder="用户名" size="large" :prefix-icon="User" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="loginForm.password" type="password" show-password placeholder="密码" size="large" :prefix-icon="Lock" @keyup.enter="onLogin" />
        </el-form-item>
        <el-button type="primary" size="large" class="submit" :loading="loading" @click="onLogin">登 录</el-button>
      </el-form>

      <el-form v-else @submit.prevent="onRegister">
        <el-form-item>
          <el-input v-model="regForm.username" placeholder="用户名（3-32 个字符）" size="large" :prefix-icon="User" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="regForm.nickname" placeholder="昵称（选填）" size="large" :prefix-icon="Postcard" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="regForm.password" type="password" show-password placeholder="密码（至少 6 位）" size="large" :prefix-icon="Lock" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="regForm.confirm" type="password" show-password placeholder="确认密码" size="large" :prefix-icon="Lock" @keyup.enter="onRegister" />
        </el-form-item>
        <el-button type="primary" size="large" class="submit" :loading="loading" @click="onRegister">注 册</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock, Postcard } from '@element-plus/icons-vue'
import { authApi } from '../api'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const tab = ref('login')
const loading = ref(false)
const loginForm = reactive({ username: '', password: '' })
const regForm = reactive({ username: '', nickname: '', password: '', confirm: '' })

onMounted(() => {
  // 默认填入上次成功登录的用户名
  const last = localStorage.getItem('last_username')
  if (last) loginForm.username = last
})

async function onLogin() {
  if (!loginForm.username || !loginForm.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(loginForm.username, loginForm.password)
    localStorage.setItem('last_username', loginForm.username.trim())
    ElMessage.success('欢迎回来！')
    router.push(route.query.redirect || '/dashboard')
  } finally {
    loading.value = false
  }
}

async function onRegister() {
  if (regForm.username.length < 3) {
    ElMessage.warning('用户名至少 3 个字符')
    return
  }
  if (regForm.password.length < 6) {
    ElMessage.warning('密码至少 6 位')
    return
  }
  if (regForm.password !== regForm.confirm) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    await authApi.register({
      username: regForm.username,
      nickname: regForm.nickname,
      password: regForm.password,
    })
    ElMessage.success('注册成功，请登录')
    tab.value = 'login'
    loginForm.username = regForm.username
    loginForm.password = ''
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: linear-gradient(135deg, #409eff 0%, #7d5fff 60%, #b06ff2 100%);
}
.login-card {
  width: 400px;
  max-width: 92vw;
  background: #fff;
  border-radius: 16px;
  padding: 32px 28px 24px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
}
.brand {
  text-align: center;
  margin-bottom: 20px;
}
.brand-icon {
  font-size: 44px;
  margin-bottom: 6px;
}
.brand h1 {
  font-size: 26px;
  color: #303133;
  letter-spacing: 2px;
}
.brand p {
  font-size: 13px;
  color: #909399;
  margin-top: 6px;
}
.tab-switch {
  display: flex;
  background: #f2f3f5;
  border-radius: 10px;
  padding: 4px;
  margin: 4px 0 20px;
}
.tab-btn {
  flex: 1;
  height: 38px;
  border: none;
  border-radius: 8px;
  background: transparent;
  font-size: 15px;
  color: #909399;
  cursor: pointer;
  transition: all 0.2s;
}
.tab-btn.active {
  background: #fff;
  color: #409eff;
  font-weight: 600;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}
.submit {
  width: 100%;
  margin-top: 8px;
}
</style>
