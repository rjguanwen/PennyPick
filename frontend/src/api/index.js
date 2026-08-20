import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const api = axios.create({
  baseURL: '/api',
  timeout: 20000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status
    const detail = error.response?.data?.detail
    if (status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login')
      }
    }
    const msg = typeof detail === 'string' ? detail : error.message || '请求失败'
    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

export default api

// ===== 认证 =====
export const authApi = {
  login: (username, password) => {
    const form = new URLSearchParams()
    form.append('username', username)
    form.append('password', password)
    return api.post('/auth/login', form)
  },
  register: (data) => api.post('/auth/register', data),
  me: () => api.get('/auth/me'),
  changePassword: (data) => api.put('/auth/password', data),
}

// ===== 分类 =====
export const categoryApi = {
  list: (type) => api.get('/categories', { params: { type } }),
  create: (data) => api.post('/categories', data),
  update: (id, data) => api.patch(`/categories/${id}`, data),
  remove: (id) => api.delete(`/categories/${id}`),
}

// ===== 账户 =====
export const accountApi = {
  list: () => api.get('/accounts'),
  create: (data) => api.post('/accounts', data),
  remove: (id) => api.delete(`/accounts/${id}`),
}

// ===== 账单 =====
export const billApi = {
  list: (params) => api.get('/bills', { params }),
  create: (data) => api.post('/bills', data),
  update: (id, data) => api.patch(`/bills/${id}`, data),
  remove: (id) => api.delete(`/bills/${id}`),
}

// ===== 预算 =====
export const budgetApi = {
  // 总预算
  get: (month) => api.get('/budgets', { params: { month } }),
  all: () => api.get('/budgets/all'),
  upsert: (data) => api.put('/budgets', data),
  remove: (month) => api.delete('/budgets', { params: { month } }),
  // 分类预算
  categories: (month) => api.get('/budgets/categories', { params: { month } }),
  upsertCategory: (data) => api.put('/budgets/category', data),
  removeCategory: (month, categoryId) =>
    api.delete('/budgets/category', { params: { month, category_id: categoryId } }),
}

// ===== 统计 =====
export const statsApi = {
  overview: (month) => api.get('/stats/overview', { params: { month } }),
  byCategory: (params) => api.get('/stats/by-category', { params }),
  trend: (params) => api.get('/stats/trend', { params }),
  accounts: (month) => api.get('/stats/accounts', { params: { month } }),
}

// ===== 导出 =====
export const exportApi = {
  // 返回 Blob，前端触发下载
  download: (params) =>
    api.get('/export', { params, responseType: 'blob' }),
}
