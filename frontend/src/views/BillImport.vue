<template>
  <div class="import-page">
    <div class="page-head">
      <span class="page-title">账单导入</span>
      <span class="page-sub">导入微信 / 支付宝导出的账单，自动识别重复记录，防止重复记账。</span>
      <el-button link type="primary" :icon="Clock" class="history-btn" @click="openHistory">导入历史</el-button>
    </div>

    <el-steps :active="step" align-center finish-status="success" class="steps" style="max-width: 720px; margin: 0 auto 20px;">
      <el-step title="选择平台" />
      <el-step title="上传文件" />
      <el-step title="预览确认" />
      <el-step title="完成" />
    </el-steps>

    <!-- Step 0 选择平台 -->
    <div v-show="step === 0" class="pp-card step-panel">
      <div class="platform-cards">
        <div class="platform-card" :class="{ active: platform === 'wechat' }" @click="selectPlatform('wechat')">
          <el-icon class="icon"><ChatDotRound /></el-icon>
          <span class="name">微信支付</span>
          <span class="sub">支持 .xlsx / .csv 账单</span>
        </div>
        <div class="platform-card" :class="{ active: platform === 'alipay' }" @click="selectPlatform('alipay')">
          <el-icon class="icon"><Wallet /></el-icon>
          <span class="name">支付宝</span>
          <span class="sub">支持 .csv 账单</span>
        </div>
      </div>
      <div class="foot">
        <el-button type="primary" size="large" :disabled="!platform" @click="step = 1">下一步</el-button>
      </div>
    </div>

    <!-- Step 1 上传文件 -->
    <div v-show="step === 1" class="pp-card step-panel">
      <div class="upload-tip">
        <p>平台：<b>{{ platformLabel }}</b></p>
        <p>请上传从 {{ platformLabel }} 导出的账单文件，文件内的记录会先解析并做去重检测，确认后再导入。</p>
      </div>

      <div class="export-guide">
        <div class="guide-title">
          <el-icon><QuestionFilled /></el-icon>
          <span>不知道如何导出账单？</span>
        </div>
        <div v-if="platform === 'wechat'" class="guide-steps">
          <ol>
            <li>打开微信 → <b>我</b> → <b>服务</b> → <b>钱包</b> → <b>账单</b></li>
            <li>点击右上角 <b>「...」</b> → 选择 <b>「下载账单」</b></li>
          </ol>
        </div>
        <div v-else class="guide-steps">
          <ol>
            <li>打开支付宝 → <b>我的</b> → <b>账单</b></li>
            <li>点击右上角 <b>「...」</b> → 选择 <b>「开具交易流水证明」</b></li>
          </ol>
        </div>
      </div>

      <el-upload
        drag
        :show-file-list="false"
        :accept="acceptStr"
        :http-request="doUpload"
        :disabled="uploading"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">将账单文件拖到此处，或<em>点击上传</em></div>
        <template #tip>
          <div class="el-upload__tip">微信：.xlsx / .xls / .csv；支付宝：.csv。文件大小不超过 10MB。</div>
        </template>
      </el-upload>
      <div class="foot">
        <el-button @click="step = 0">上一步</el-button>
      </div>
    </div>

    <!-- Step 2 预览确认 -->
    <div v-show="step === 2" class="preview-wrap">
      <el-alert
        v-if="parseData && (parseData.duplicates > 0 || parseData.filtered > 0)"
        :type="parseData.duplicates > 0 ? 'warning' : 'info'"
        :closable="false"
        show-icon
        class="alert"
      >
        <template #title>
          <span v-if="parseData.duplicates > 0">发现 {{ parseData.duplicates }} 条重复记录，已默认不勾选。</span>
          <span v-if="parseData.filtered > 0">已自动过滤 {{ parseData.filtered }} 条理财/投资或退款记录。</span>
        </template>
      </el-alert>

      <div class="stat-cards">
        <div class="stat-card"><span class="num">{{ parseData?.total_count || 0 }}</span><span class="label">总条数</span></div>
        <div class="stat-card"><span class="num exp">¥{{ formatMoney(expenseTotal) }}</span><span class="label">支出</span></div>
        <div class="stat-card"><span class="num inc">¥{{ formatMoney(incomeTotal) }}</span><span class="label">收入</span></div>
        <div class="stat-card warn"><span class="num">{{ parseData?.duplicates || 0 }}</span><span class="label">重复</span></div>
        <div class="stat-card dim"><span class="num">{{ parseData?.filtered || 0 }}</span><span class="label">已过滤</span></div>
      </div>

      <div class="pp-card toolbar-card">
        <div class="tool-row">
          <span class="label">默认账户</span>
          <el-select v-model="defaultAccountId" placeholder="选择账户" style="width: 200px" @change="applyAccountToAll">
            <el-option v-for="a in accounts" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
          <span class="label sep">分类匹配</span>
          <el-button size="small" :icon="MagicStick" @click="smartMatchAll">智能匹配分类</el-button>
          <span class="label sep">重复处理</span>
          <el-checkbox v-model="skipDuplicates">跳过重复记录</el-checkbox>
        </div>
      </div>

      <div class="pp-card table-card">
        <el-table
          ref="tableRef"
          :data="items"
          height="480"
          border
          :row-class-name="rowClass"
          @selection-change="onSelectionChange"
        >
          <el-table-column type="selection" width="44" :selectable="rowSelectable" />
          <el-table-column label="状态" width="86" fixed="left">
            <template #default="{ row }">
              <el-tag v-if="row.is_filtered" type="info" size="small">已过滤</el-tag>
              <el-tag v-else-if="row.is_duplicate" type="danger" size="small">{{ row.duplicate_way === 'order_no' ? '重复订单' : '重复记录' }}</el-tag>
              <el-tag v-else type="success" size="small">新记录</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="日期" width="130" prop="occurred_at" />
          <el-table-column label="类型" width="64">
            <template #default="{ row }">
              <el-tag :type="row.type === 'income' ? 'success' : 'danger'" size="small" effect="plain">
                {{ row.type === 'income' ? '收入' : '支出' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="金额" width="100" align="right">
            <template #default="{ row }">
              <span class="amount">{{ formatMoney(row.amount) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="交易对方" width="150" show-overflow-tooltip prop="counterparty" />
          <el-table-column label="支付方式" width="160" show-overflow-tooltip>
            <template #default="{ row }">{{ row.pay_way || '—' }}</template>
          </el-table-column>
          <el-table-column label="备注" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.note || '—' }}</template>
          </el-table-column>
          <el-table-column label="账户" width="160">
            <template #default="{ row }">
              <el-select v-model="row.account_id" size="small" placeholder="选择账户">
                <el-option v-for="a in accounts" :key="a.id" :label="a.name" :value="a.id" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="150">
            <template #default="{ row }">
              <el-select v-model="row.category_id" size="small" placeholder="分类" filterable>
                <el-option v-for="c in categoriesByType(row.type)" :key="c.id" :label="c.name" :value="c.id" />
              </el-select>
            </template>
          </el-table-column>
        </el-table>
        <div class="foot-bar">
          <span class="hint">已勾选 <b>{{ selectedCount }}</b> 笔，将导入 <b>¥{{ formatMoney(selectedTotal) }}</b></span>
          <div class="foot-btns">
            <el-button @click="step = 1">重新上传</el-button>
            <el-button type="primary" size="large" :loading="confirming" :disabled="selectedCount === 0" @click="doConfirm">确认导入</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- Step 3 完成 -->
    <div v-show="step === 3" class="pp-card step-panel">
      <el-result icon="success" :title="`成功导入 ${result?.imported_count || 0} 笔账单`" :sub-title="result?.message">
        <template #extra>
          <div class="result-extra">
            <el-button type="primary" @click="resetAll">再导入一份</el-button>
            <el-button @click="router.push('/bills')">查看账单</el-button>
          </div>
        </template>
      </el-result>
    </div>

    <!-- 导入历史 -->
    <el-dialog v-model="showHistory" title="导入历史" width="760px" :append-to-body="true">
      <template v-if="!historyDetail">
        <el-table :data="historyList" v-loading="historyLoading">
          <el-table-column label="时间" width="160" prop="created_at" />
          <el-table-column label="平台" width="90">
            <template #default="{ row }">{{ row.platform === 'wechat' ? '微信' : '支付宝' }}</template>
          </el-table-column>
          <el-table-column label="文件名" min-width="180" show-overflow-tooltip prop="file_name" />
          <el-table-column label="总条数" width="80" align="right" prop="total_count" />
          <el-table-column label="导入" width="80" align="right">
            <template #default="{ row }"><span style="color:#67c23a">{{ row.imported_count }}</span></template>
          </el-table-column>
          <el-table-column label="跳过" width="80" align="right">
            <template #default="{ row }"><span style="color:#909399">{{ row.skipped_count }}</span></template>
          </el-table-column>
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" size="small" @click="viewHistoryDetail(row.id)">明细</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="pager">
          <el-pagination
            layout="prev, pager, next, total"
            :total="historyTotal"
            :page-size="historyPageSize"
            :current-page="historyPage"
            @current-change="loadHistory"
          />
        </div>
      </template>
      <template v-else>
        <div class="detail-head">
          <el-button link type="primary" :icon="ArrowLeft" @click="historyDetail = null">返回</el-button>
          <span class="detail-title">{{ historyDetail.platform === 'wechat' ? '微信' : '支付宝' }} · {{ historyDetail.file_name }}</span>
        </div>
        <el-table :data="detailItems" height="420" border>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="row.status === 'imported' ? 'success' : 'info'" size="small">
                {{ row.status === 'imported' ? '已导入' : '已跳过' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="日期" width="130" prop="occurred_at" />
          <el-table-column label="类型" width="64">
            <template #default="{ row }">
              <el-tag v-if="row.type" :type="row.type === 'income' ? 'success' : 'danger'" size="small" effect="plain">{{ row.type === 'income' ? '收入' : '支出' }}</el-tag>
              <span v-else>—</span>
            </template>
          </el-table-column>
          <el-table-column label="金额" width="100" align="right">
            <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
          </el-table-column>
          <el-table-column label="交易对方" width="140" show-overflow-tooltip prop="counterparty" />
          <el-table-column label="订单号" min-width="200" show-overflow-tooltip prop="platform_order_no" />
          <el-table-column label="原因" min-width="140" show-overflow-tooltip prop="skip_reason" />
        </el-table>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { ArrowLeft, ChatDotRound, Clock, MagicStick, UploadFilled, Wallet } from '@element-plus/icons-vue'
import { accountApi, billImportApi, categoryApi } from '../api'
import { formatMoney } from '../utils/format'

const router = useRouter()
const step = ref(0)
const platform = ref('')
const platformLabel = computed(() => (platform.value === 'wechat' ? '微信支付' : '支付宝'))
const acceptStr = computed(() => (platform.value === 'wechat' ? '.xlsx,.xls,.csv' : '.csv'))
const uploading = ref(false)
const fileName = ref('')
const parseData = ref(null)
const items = ref([])
const accounts = ref([])
const categories = ref([])
const defaultAccountId = ref(null)
const skipDuplicates = ref(true)
const selectedRows = ref([])
const confirming = ref(false)
const result = ref(null)

const selectedCount = computed(() => selectedRows.value.length)
const selectedTotal = computed(() => selectedRows.value.reduce((s, r) => s + Number(r.amount || 0), 0))
const expenseTotal = computed(() => items.value.filter((i) => i.type === 'expense' && !i.is_filtered).reduce((s, i) => s + i.amount, 0))
const incomeTotal = computed(() => items.value.filter((i) => i.type === 'income' && !i.is_filtered).reduce((s, i) => s + i.amount, 0))

function selectPlatform(p) {
  platform.value = p
  step.value = 1
}

async function doUpload({ file }) {
  if (platform.value === 'wechat' && !/\.(xlsx|xls|csv)$/i.test(file.name)) {
    ElMessage.warning('微信账单请上传 .xlsx / .xls / .csv 文件')
    return
  }
  if (platform.value === 'alipay' && !/\.csv$/i.test(file.name)) {
    ElMessage.warning('支付宝账单请上传 .csv 文件')
    return
  }
  uploading.value = true
  fileName.value = file.name
  try {
    const res = await billImportApi.parse(file, platform.value)
    parseData.value = res
    items.value = (res.items || []).map((it) => ({
      ...it,
      account_id: matchAccount(it.pay_way) || null,
      category_id: smartCategory(it) || null,
    }))
    if (items.value.length === 0) {
      ElMessage.warning('文件中没有识别到可导入的账单记录，请检查文件内容')
      return
    }
    step.value = 2
  } catch (e) {
    // 拦截器已提示
  } finally {
    uploading.value = false
  }
}

// 账户自动匹配：按支付方式文本匹配已有账户
function matchAccount(payWay) {
  if (!payWay || !accounts.value.length) return null
  const accs = accounts.value
  for (const a of accs) {
    if (a.name.length >= 2 && payWay.includes(a.name)) return a.id
  }
  const kwList = ['微信', '支付宝', '银行卡', '招商', '工商', '农业', '建设', '交通', '民生', '零钱', '现金']
  for (const a of accs) {
    for (const k of kwList) {
      if (payWay.includes(k) && a.name.includes(k)) return a.id
    }
  }
  return accs.length ? accs[0].id : null
}

// 分类智能匹配：备注/对方包含分类名则命中，否则兜底「其他」
function smartCategory(item) {
  const cats = (categories.value || []).filter((c) => c.type === item.type)
  if (!cats.length) return null
  const text = `${item.note || ''} ${item.counterparty || ''}`
  for (const c of cats) {
    if (c.name !== '其他' && text.includes(c.name)) return c.id
  }
  const other = cats.find((c) => c.name === '其他')
  return other ? other.id : cats[0].id
}

function smartMatchAll() {
  items.value.forEach((it) => {
    it.category_id = smartCategory(it) || it.category_id
  })
  ElMessage.success('已按备注关键词重新匹配分类')
}

function applyAccountToAll() {
  if (!defaultAccountId.value) return
  items.value.forEach((it) => {
    it.account_id = defaultAccountId.value
  })
}

function categoriesByType(type) {
  return (categories.value || []).filter((c) => c.type === type)
}

function rowSelectable(row) {
  if (row.is_filtered) return false
  if (skipDuplicates.value && row.is_duplicate) return false
  return true
}

function rowClass({ row }) {
  if (row.is_filtered) return 'row-filtered'
  if (row.is_duplicate) return 'row-duplicate'
  return ''
}

function onSelectionChange(rows) {
  selectedRows.value = rows
}

async function doConfirm() {
  confirming.value = true
  try {
    const payload = {
      platform: platform.value,
      file_name: fileName.value,
      account_id: defaultAccountId.value || 0,
      items: items.value.map((it) => ({
        platform_order_no: it.platform_order_no,
        occurred_at: it.occurred_at,
        amount: it.amount,
        type: it.type,
        counterparty: it.counterparty,
        note: it.note,
        pay_way: it.pay_way,
        category_id: it.category_id || 0,
        account_id: it.account_id || 0,
        is_duplicate: it.is_duplicate,
        duplicate_way: it.duplicate_way,
        is_filtered: it.is_filtered,
        filter_reason: it.filter_reason,
        selected: selectedRows.value.includes(it),
      })),
    }
    const res = await billImportApi.confirm(payload)
    result.value = res
    step.value = 3
  } catch (e) {
    // 拦截器已提示
  } finally {
    confirming.value = false
  }
}

function resetAll() {
  step.value = 0
  platform.value = ''
  parseData.value = null
  items.value = []
  selectedRows.value = []
  result.value = null
  fileName.value = ''
  defaultAccountId.value = null
}

// ===== 导入历史 =====
const showHistory = ref(false)
const historyList = ref([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyPageSize = 10
const historyLoading = ref(false)
const historyDetail = ref(null)
const detailItems = ref([])

async function openHistory() {
  showHistory.value = true
  historyDetail.value = null
  await loadHistory(1)
}

async function loadHistory(page = historyPage.value) {
  historyPage.value = page
  historyLoading.value = true
  try {
    const res = await billImportApi.history(page, historyPageSize)
    historyList.value = res.items || []
    historyTotal.value = res.total || 0
  } finally {
    historyLoading.value = false
  }
}

async function viewHistoryDetail(id) {
  const res = await billImportApi.detail(id)
  historyDetail.value = res.import
  detailItems.value = res.items || []
}

onMounted(async () => {
  const [accs, cats] = await Promise.all([accountApi.list(), categoryApi.list()])
  accounts.value = accs || []
  categories.value = cats || []
  if (accounts.value.length) {
    defaultAccountId.value = accounts.value[0].id
  }
})
</script>

<style scoped>
.import-page {
  /* 铺满可显示区（同账户还款页），便于多列表格一次展示 */
}
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
.history-btn {
  margin-left: auto;
}
.step-panel {
  padding: 32px;
  min-height: 300px;
}
.platform-cards {
  display: flex;
  gap: 24px;
  justify-content: center;
  padding: 30px 0 40px;
}
.platform-card {
  width: 220px;
  border: 2px solid #e4e7ed;
  border-radius: 12px;
  padding: 26px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s;
}
.platform-card:hover {
  border-color: #a0cfff;
  transform: translateY(-2px);
}
.platform-card.active {
  border-color: #409eff;
  background: #ecf5ff;
}
.platform-card .icon {
  font-size: 40px;
  color: #409eff;
}
.platform-card .name {
  font-size: 16px;
  font-weight: 600;
}
.platform-card .sub {
  font-size: 12px;
  color: #909399;
}
.foot {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}
.upload-tip {
  margin-bottom: 14px;
  font-size: 13px;
  color: #606266;
  line-height: 1.8;
}
.upload-tip p {
  margin: 0;
}
.export-guide {
  background: #f4f8ff;
  border: 1px dashed #a0cfff;
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 18px;
}
.guide-title {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #409eff;
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 6px;
}
.guide-steps ol {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #606266;
  line-height: 2;
}
.guide-steps b {
  color: #303133;
}
.alert {
  margin-bottom: 14px;
}
.stat-cards {
  display: flex;
  gap: 12px;
  margin-bottom: 14px;
}
.stat-card {
  flex: 1;
  background: #fff;
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.stat-card .num {
  font-size: 20px;
  font-weight: 700;
  color: #303133;
}
.stat-card .num.exp {
  color: #f56c6c;
}
.stat-card .num.inc {
  color: #67c23a;
}
.stat-card.warn .num {
  color: #e6a23c;
}
.stat-card.dim .num {
  color: #909399;
}
.stat-card .label {
  font-size: 12px;
  color: #909399;
}
.toolbar-card {
  margin-bottom: 14px;
  padding: 12px 16px;
}
.tool-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.label {
  font-size: 14px;
  color: #606266;
}
.label.sep {
  margin-left: 16px;
}
.table-card {
  padding: 16px;
}
.amount {
  font-weight: 600;
}
.foot-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
}
.hint {
  font-size: 13px;
  color: #606266;
}
.hint b {
  color: #303133;
}
.result-extra {
  display: flex;
  justify-content: center;
  gap: 12px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.detail-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.detail-title {
  font-weight: 600;
}
:deep(.row-filtered) {
  background: #fafafa;
  color: #c0c4cc;
}
:deep(.row-filtered .el-select) {
  pointer-events: none;
}
:deep(.row-duplicate) {
  background: #fef0f0;
}
</style>
