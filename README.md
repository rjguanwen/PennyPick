# 拾财 PennyPick

个人记账应用：轻松记下每一笔消费，按月设置预算预警，多维度统计分析，支持账单导出。桌面端（PC）风格界面。

## 功能简介

1. **快速记账**：大数字键盘输入、常用分类置顶、支持「保存后继续记」，两三秒完成一笔；收入/支出一键切换。
2. **月度预算预警**：支持**总预算**与**分类预算**两档（分类预算如「餐饮每月 800 元」），均按月份设置金额与预警阈值（如 80%），达到阈值或超支时在预算页、首页、记账页及时提醒。
3. **多维度统计**：分类占比饼图、按日/按月收支趋势、账户收支分布、月度概览（支出/收入/结余/日均/笔数）。
4. **账单管理**：按月份、类型、分类、关键字筛选账单；按日分组展示；支持编辑与删除。
5. **账单导出**：导出 CSV（UTF-8 BOM，Excel 可直接打开），支持时间范围与类型筛选。
6. **分类与账户**：预置常用收支分类与账户，可自定义分类名称、图标、颜色。
7. **多用户**：支持注册多个账号，每个用户数据相互隔离（默认账号 `admin / admin123`）。

## 技术栈

- 前端：Vue 3 + Element Plus + Vite + Pinia + Vue Router + ECharts（端口 5175）
- 后端：Go（Gin + GORM），数据库 SQLite（端口 8003）
- 认证：JWT（Bearer Token，有效期 30 天）

## 目录结构

```
pennypick/
├── backend-go/          # Go 后端
│   ├── cmd/server/      # 入口
│   └── internal/
│       ├── config/      # 配置（环境变量）
│       ├── database/    # SQLite 连接、迁移、默认数据初始化
│       ├── model/       # 数据模型（用户/分类/账户/账单/预算）
│       ├── middleware/  # JWT 认证、CORS
│       └── handler/     # 业务处理（认证/分类/账户/账单/预算/统计/导出）
└── frontend/            # Vue 前端
    └── src/
        ├── api/         # axios 封装
        ├── router/      # 路由
        ├── stores/      # Pinia
        ├── utils/       # 格式化工具
        ├── layout/      # 响应式布局（PC 侧边栏 / 手机底部导航）
        ├── components/  # 通用组件
        └── views/       # 页面（首页/记账/账单/统计/预算/分类/设置）
```

## 快速启动

### 1. 启动后端

```bash
cd backend-go
go run ./cmd/server
```

默认监听 `:8003`，首次启动自动建库建表、创建默认账号并初始化预置分类/账户。

可用环境变量（`.env` 或系统环境变量）：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8003` | 后端端口 |
| `DATABASE_URL` | `sqlite:///./pennypick.db` | SQLite 文件路径 |
| `SECRET_KEY` | 随机串 | JWT 签名密钥（生产必须修改） |
| `INIT_ADMIN_USERNAME` | `admin` | 初始用户名 |
| `INIT_ADMIN_PASSWORD` | `admin123` | 初始密码 |
| `INIT_ADMIN_NICKNAME` | `主人` | 初始昵称 |

### 2. 启动前端

```bash
cd frontend
npm install
npm run dev
```

访问 `http://localhost:5175`。前端通过 Vite 代理将 `/api` 转发至后端 `8003`（可用 `VITE_BACKEND_PORT` 覆盖）。

### 3. 使用流程

1. **登录**：`admin / admin123`（或先注册新账号）
2. **记一笔**：左侧菜单「记一笔」进入记账页 → 选支出/收入 → 点数字键盘输入金额 → 选分类 → 保存（可勾选「保存后继续记」）
3. **设预算**：预算页选择月份 → 设置总预算（金额 + 预警阈值），或为单个支出分类设置分类预算（如餐饮 800 元/月）→ 保存；首页与记账页自动预警
4. **看统计**：统计页切换月份，查看分类占比、收支趋势、账户分布
5. **导账单**：设置页选择时间范围与类型 → 导出 CSV
6. **管分类**：分类管理页自定义分类的图标与颜色

## 数据模型

| 表 | 说明 |
| --- | --- |
| `users` | 用户（个人记账，多用户数据隔离） |
| `categories` | 分类（expense 支出 / income 收入，含图标与颜色） |
| `accounts` | 账户（现金/微信/支付宝/银行卡等） |
| `bills` | 账单（类型、金额、分类、账户、发生时间、备注） |
| `budgets` | 月度总预算（唯一索引 user_id + month，含预警阈值） |
| `category_budgets` | 分类预算（唯一索引 user_id + month + category_id，独立预警） |

## 主要接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/login` | 登录（form-data） |
| POST | `/api/auth/register` | 注册（自动初始化预置分类/账户） |
| GET | `/api/auth/me` | 当前用户 |
| PUT | `/api/auth/password` | 修改密码 |
| GET | `/api/categories` | 分类列表（`type` 过滤，含近30天使用次数） |
| POST/PATCH/DELETE | `/api/categories[/:id]` | 分类增改删 |
| GET | `/api/accounts` | 账户列表 |
| POST/DELETE | `/api/accounts[/:id]` | 账户增删 |
| GET | `/api/bills` | 账单列表（month/start/end/type/category_id/account_id/keyword + 分页） |
| POST | `/api/bills` | 记一笔 |
| PATCH/DELETE | `/api/bills/:id` | 账单改/删 |
| GET/PUT/DELETE | `/api/budgets` | 月度总预算查询/设置/删除 |
| GET | `/api/budgets/all` | 全部总预算 |
| GET | `/api/budgets/categories` | 某月各支出分类的预算与已用情况 |
| PUT | `/api/budgets/category` | 设置/更新分类预算 |
| DELETE | `/api/budgets/category` | 删除分类预算 |
| GET | `/api/stats/overview` | 月度概览（含预算进度与预警状态） |
| GET | `/api/stats/by-category` | 分类统计 |
| GET | `/api/stats/trend` | 收支趋势（按月/按日） |
| GET | `/api/stats/accounts` | 账户收支统计 |
| GET | `/api/export` | 导出 CSV（start/end 支持 YYYY-MM-DD 或 YYYY-MM） |

完整接口见 `backend-go/internal/handler/handler.go`。
