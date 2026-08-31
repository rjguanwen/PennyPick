import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('../layout/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '首页' } },
      { path: 'record', name: 'record', component: () => import('../views/Record.vue'), meta: { title: '记一笔' } },
      { path: 'batch-record', name: 'batch-record', component: () => import('../views/BatchRecord.vue'), meta: { title: '批量记账' } },
      { path: 'recurring-bills', name: 'recurring-bills', component: () => import('../views/RecurringBill.vue'), meta: { title: '固单记账' } },
      { path: 'bill-import', name: 'bill-import', component: () => import('../views/BillImport.vue'), meta: { title: '账单导入' } },
      { path: 'bills', name: 'bills', component: () => import('../views/BillList.vue'), meta: { title: '账单查询' } },
      { path: 'repayment', name: 'repayment', component: () => import('../views/Repayment.vue'), meta: { title: '账户还款' } },
      { path: 'stats', name: 'stats', component: () => import('../views/Stats.vue'), meta: { title: '统计分析' } },
      { path: 'reports', name: 'reports', component: () => import('../views/MonthlyReport.vue'), meta: { title: '收支报告' } },
      { path: 'oplogs', name: 'oplogs', component: () => import('../views/OpLogs.vue'), meta: { title: '操作日志' } },
      { path: 'account-query', name: 'account-query', component: () => import('../views/AccountQuery.vue'), meta: { title: '账户查询' } },
      { path: 'budget', name: 'budget', component: () => import('../views/Budget.vue'), meta: { title: '预算管理' } },
      { path: 'categories', name: 'categories', component: () => import('../views/CategoryManage.vue'), meta: { title: '分类管理' } },
      { path: 'accounts', name: 'accounts', component: () => import('../views/AccountManage.vue'), meta: { title: '账户管理' } },
      { path: 'settings', name: 'settings', component: () => import('../views/Settings.vue'), meta: { title: '设置' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && token) {
    return { name: 'dashboard' }
  }
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · 拾财 PennyPick` : '拾财 · PennyPick'
})

export default router
