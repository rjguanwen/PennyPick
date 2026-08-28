<template>
  <div class="oplog-page">
    <div class="page-head">
      <span class="page-title">操作日志</span>
      <span class="page-sub">记录登录、记账、修改、删除等重要操作（数据量较大，请按需筛选）</span>
    </div>

    <!-- 筛选 -->
    <div class="filter-bar">
      <el-input v-model="filters.action" placeholder="操作类型，如：记一笔" clearable style="width: 180px" @keyup.enter="search" />
      <el-input v-model="filters.username" placeholder="用户名" clearable style="width: 150px" @keyup.enter="search" />
      <el-date-picker
        v-model="range"
        type="datetimerange"
        value-format="YYYY-MM-DD HH:mm:ss"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        style="width: 340px"
      />
      <el-button type="primary" :icon="Search" @click="search">查询</el-button>
      <el-button :icon="Refresh" @click="reset">重置</el-button>
    </div>

    <!-- 列表 -->
    <div class="pp-card">
      <el-table :data="items" size="small" style="width: 100%" v-loading="loading">
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="username" label="用户" width="110">
          <template #default="{ row }">{{ row.username || '—' }}</template>
        </el-table-column>
        <el-table-column prop="action" label="操作" width="120" />
        <el-table-column prop="path" label="接口" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column label="结果" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.status < 300" size="small" type="success">成功</el-tag>
            <el-tag v-else size="small" type="danger">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!items.length && !loading" description="暂无操作日志" :image-size="60" />

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="onPage"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Search, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { oplogApi } from '../api'

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const filters = reactive({ action: '', username: '' })
const range = ref(null)

function fmtTime(t) {
  return t ? String(t).slice(0, 19).replace('T', ' ') : ''
}

async function load() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize, ...filters }
    if (range.value && range.value.length === 2) {
      params.start = range.value[0]
      params.end = range.value[1]
    }
    const res = await oplogApi.list(params)
    items.value = res.items || []
    total.value = res.total || 0
  } catch (e) {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}
function reset() {
  filters.action = ''
  filters.username = ''
  range.value = null
  page.value = 1
  load()
}
function onPage(p) {
  page.value = p
  load()
}

onMounted(load)
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 14px;
}
.page-title {
  font-size: 17px;
  font-weight: 700;
}
.page-sub {
  font-size: 12px;
  color: #909399;
}
.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
