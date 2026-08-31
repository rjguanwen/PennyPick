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
  update: (id, data) => api.patch(`/accounts/${id}`, data),
  remove: (id) => api.delete(`/accounts/${id}`),
}

// ===== 还款 =====
export const repaymentApi = {
  list: (month) => api.get('/repayments', { params: { month } }),
  bills: (month, accountId) => api.get('/repayments/bills', { params: { month, account_id: accountId } }),
  mark: (data) => api.post('/repayments', data),
  unmark: (month, accountId) => api.delete('/repayments', { params: { month, account_id: accountId } }),
}

// ===== 标签 =====
export const tagApi = {
  list: () => api.get('/tags'),
  create: (data) => api.post('/tags', data),
  update: (id, data) => api.patch(`/tags/${id}`, data),
  remove: (id) => api.delete(`/tags/${id}`),
}

// ===== 账单 =====
export const billApi = {
  list: (params) => api.get('/bills', { params }),
  create: (data) => api.post('/bills', data),
  createBatch: (data) => api.post('/bills/batch', data),
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
  // 复制预算：把某月预算复制到另一月（覆盖目标月）
  copy: (data) => api.post('/budgets/copy', data),
}

// ===== 固定账单 =====
export const recurringBillApi = {
  list: () => api.get('/recurring-bills'),
  create: (data) => api.post('/recurring-bills', data),
  update: (id, data) => api.patch(`/recurring-bills/${id}`, data),
  remove: (id) => api.delete(`/recurring-bills/${id}`),
  apply: (data) => api.post('/recurring-bills/apply', data),
}

// ===== 账户查询（多账户按月支出）=====
export const accountQueryApi = {
  query: (params) => api.get('/accounts/query', { params }),
  bills: (params) => api.get('/accounts/query/bills', { params }),
}

// ===== 操作日志（仅管理员）=====
export const oplogApi = {
  setting: () => api.get('/settings/oplog'),
  setEnabled: (enabled) => api.put('/settings/oplog', { enabled }),
  list: (params) => api.get('/oplogs', { params }),
}

// ===== 收支报告（月度 + 年度）=====
export const reportsApi = {
  list: () => api.get('/reports'),
  generate: (data) => api.post('/reports/generate', data),
  detail: (id) => api.get(`/reports/${id}`),
  yearlyGenerate: (data) => api.post('/reports/yearly', data),
  yearlyList: () => api.get('/reports/yearly/list'),
  yearlyDetail: (id) => api.get(`/reports/yearly/${id}`),
}

// ===== 统计 =====
export const statsApi = {
  overview: (month) => api.get('/stats/overview', { params: { month } }),
  byCategory: (params) => api.get('/stats/by-category', { params }),
  trend: (params) => api.get('/stats/trend', { params }),
  accounts: (month) => api.get('/stats/accounts', { params: { month } }),
  tags: (params) => api.get('/stats/tags', { params }),
}

// ===== 导出 =====
export const exportApi = {
  // 返回 Blob，前端触发下载
  download: (params) =>
    api.get('/export', { params, responseType: 'blob' }),
}

// ===== 账单导入 =====
export const billImportApi = {
  // 上传账单文件并解析（预览）
  parse: (file, platform) => {
    const form = new FormData()
    form.append('file', file)
    form.append('platform', platform)
    return api.post('/bill-import/parse', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000,
    })
  },
  // 确认导入
  confirm: (data) => api.post('/bill-import/confirm', data),
  // 导入历史
  history: (page = 1, pageSize = 10) => api.get('/bill-import/history', { params: { page, page_size: pageSize } }),
  // 导入明细
  detail: (id) => api.get(`/bill-import/${id}`),
}
