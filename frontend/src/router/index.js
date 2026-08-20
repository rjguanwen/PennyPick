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
      { path: 'bills', name: 'bills', component: () => import('../views/BillList.vue'), meta: { title: '账单' } },
      { path: 'stats', name: 'stats', component: () => import('../views/Stats.vue'), meta: { title: '统计' } },
      { path: 'budget', name: 'budget', component: () => import('../views/Budget.vue'), meta: { title: '预算' } },
      { path: 'categories', name: 'categories', component: () => import('../views/CategoryManage.vue'), meta: { title: '分类管理' } },
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
