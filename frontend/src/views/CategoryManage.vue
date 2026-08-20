<template>
  <div class="category-page">
    <div class="head">
      <el-radio-group v-model="type" size="large" @change="load">
        <el-radio-button value="expense">支出分类</el-radio-button>
        <el-radio-button value="income">收入分类</el-radio-button>
      </el-radio-group>
      <el-button type="primary" :icon="Plus" @click="openDialog()">新增分类</el-button>
    </div>

    <div class="pp-card">
      <el-skeleton v-if="loading" :rows="6" animated />
      <el-empty v-else-if="!list.length" description="暂无分类" :image-size="70" />
      <div v-else class="cat-grid">
        <div v-for="cat in list" :key="cat.id" class="cat-card">
          <CatIcon :icon="cat.icon" :color="cat.color" :size="22" />
          <div class="cat-info">
            <div class="cat-name">
              {{ cat.name }}
              <el-tag v-if="cat.name === '其他'" type="info" size="small" effect="plain">内置</el-tag>
            </div>
            <div class="cat-used">近30天 {{ cat.recent_count || 0 }} 次</div>
          </div>
          <div class="cat-ops">
            <template v-if="cat.name !== '其他'">
              <el-button link type="primary" size="small" :icon="Edit" @click="openDialog(cat)" />
              <el-button link type="danger" size="small" :icon="Delete" @click="remove(cat)" />
            </template>
          </div>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑分类' : '新增分类'" width="min(420px, 92vw)">
      <el-form label-position="top">
        <el-form-item label="分类名称">
          <el-input v-model="form.name" maxlength="32" placeholder="请输入分类名称" />
        </el-form-item>
        <el-form-item label="图标">
          <div class="icon-grid">
            <button
              v-for="ic in iconOptions"
              :key="ic"
              type="button"
              class="icon-opt pp-tap"
              :class="{ selected: form.icon === ic }"
              @click="form.icon = ic"
            >
              <el-icon :size="20"><component :is="ic" /></el-icon>
            </button>
          </div>
        </el-form-item>
        <el-form-item label="颜色">
          <div class="color-grid">
            <button
              v-for="c in colorOptions"
              :key="c"
              type="button"
              class="color-opt pp-tap"
              :class="{ selected: form.color === c }"
              :style="{ background: c }"
              @click="form.color = c"
            >
              <el-icon v-if="form.color === c" color="#fff"><Check /></el-icon>
            </button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Check } from '@element-plus/icons-vue'
import * as ElementPlusIcons from '@element-plus/icons-vue'
import { categoryApi } from '../api'
import CatIcon from '../components/CatIcon.vue'

const type = ref('expense')
const list = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const form = reactive({ name: '', icon: 'More', color: '#909399' })

// 精选图标：仅保留图标库中真实存在的名字，防止动态渲染无效组件
const iconPicks = [
  'Dish', 'CoffeeCup', 'IceCream', 'MilkTea', 'Orange', 'Apple', 'Cherry', 'Grape',
  'Pear', 'Watermelon', 'GobletFull', 'Sugar', 'ShoppingBag', 'ShoppingCart', 'ShoppingTrolley',
  'Wallet', 'CreditCard', 'Goods', 'Coin', 'Money', 'PriceTag', 'Tickets', 'Postcard',
  'Van', 'Bicycle', 'Position', 'Promotion', 'Ship', 'House', 'HomeFilled', 'Key', 'OfficeBuilding',
  'Film', 'VideoCamera', 'VideoPlay', 'Headset', 'Microphone', 'Monitor', 'Iphone', 'Cellphone',
  'Basketball', 'Football', 'Trophy', 'Medal', 'FirstAidKit', 'Reading', 'Notebook', 'School',
  'Present', 'Suitcase', 'MapLocation', 'Sunny', 'Moon', 'Star', 'Clock', 'Watch',
  'Brush', 'Scissor', 'Umbrella', 'Bell', 'Camera', 'Picture', 'EditPen', 'Memo',
  'Document', 'CopyDocument', 'Calendar', 'Lightning', 'TrendCharts', 'Histogram', 'DataLine',
  'Odometer', 'Compass', 'Tools', 'SetUp', 'Setting', 'Grid', 'Menu', 'More',
]
const iconOptions = iconPicks.filter((n) => n in ElementPlusIcons)

const colorOptions = [
  '#f56c6c', '#e6a23c', '#f7ba2a', '#67c23a', '#409eff',
  '#7e57c2', '#00b4d8', '#ff6b35', '#e6607a', '#909399',
]

async function load() {
  loading.value = true
  try {
    list.value = (await categoryApi.list(type.value)) || []
  } finally {
    loading.value = false
  }
}

function openDialog(cat) {
  editing.value = cat || null
  form.name = cat?.name || ''
  form.icon = cat?.icon || 'More'
  form.color = cat?.color || '#409eff'
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入分类名称')
    return
  }
  saving.value = true
  try {
    const payload = { name: form.name.trim(), type: type.value, icon: form.icon, color: form.color }
    if (editing.value) {
      await categoryApi.update(editing.value.id, payload)
    } else {
      await categoryApi.create(payload)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function remove(cat) {
  try {
    await ElMessageBox.confirm(`确定删除分类「${cat.name}」吗？`, '删除分类', { type: 'warning' })
  } catch (e) {
    return
  }
  try {
    await categoryApi.remove(cat.id)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<style scoped>
.category-page {
  max-width: 900px;
  margin: 0 auto;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 8px;
}
.cat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
}
.cat-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 10px;
}
.cat-info {
  flex: 1;
  min-width: 0;
}
.cat-name {
  font-size: 15px;
  font-weight: 600;
}
.cat-used {
  font-size: 12px;
  color: #c0c4cc;
}
.cat-ops {
  display: flex;
}
.icon-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 8px;
}
.icon-opt {
  height: 40px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
  color: #606266;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}
.icon-opt.selected {
  border-color: #409eff;
  color: #409eff;
  background: #ecf5ff;
}
.color-grid {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.color-opt {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  border: 2px solid transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}
.color-opt.selected {
  border-color: #303133;
}
</style>
