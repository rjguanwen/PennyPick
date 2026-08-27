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
│   ├── cmd/
│   │   ├── server/      # 服务入口
│   │   ├── dbview/      # 运维查看工具（解密查看 / dump SQL）
│   │   └── dbencrypt/   # 加密运维工具（加密 / 解密 / 修改密码）
│   └── internal/
│       ├── config/      # 配置（环境变量）
│       ├── crypto/      # 整库加解密（AES-256-GCM + Argon2id）
│       ├── database/    # SQLite 连接、加密生命周期、迁移、默认数据初始化
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

默认监听 `:8003`，首次启动自动建库建表、创建默认账号并初始化预置分类/账户。**如需启用数据库加密，请设置 `PENNYPICK_DB_PASS` 环境变量（见下文「数据库加密与运维」）。**

可用环境变量（`.env` 或系统环境变量）：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8003` | 后端端口 |
| `DATABASE_URL` | `sqlite:///./pennypick.db` | SQLite 文件路径 |
| `PENNYPICK_DB_PASS` | 空 | 数据库主密码。**设置后启用整库加密**；首次带密码启动自动把明文库迁移为 `.enc`，之后每次启动以此密码解锁 |
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

## 数据库加密与运维

### 为什么加密

SQLite 数据库文件本身没有密码保护，任何人拿到 `pennypick.db` 后都可以用 Navicat / DBeaver 等客户端直接查看全部账本数据。拾财后端采用**应用层 AES-256-GCM 整库加密**：加密后落盘文件为 `pennypick.db.enc`，属于自定义封装格式，常见数据库客户端无法识别，即使拷贝走也无法查看。

### 加密原理

- **密钥派生**：主密码经 **Argon2id**（64MB 内存 / 3 轮 / 4 并行）派生 32 字节 AES 密钥；主密码本身不落盘、不随文件保存。
- **加密文件格式**：`magic(8B) | version(2B) | salt(16B) | nonce(12B) | AES-GCM 密文`，每次加密随机生成 salt 与 nonce。
- **运行机制**：启动时用密码解密 `.enc` 到系统临时目录的临时明文文件供程序读取；运行中每 5 分钟自动回写加密一次；正常退出时回写 `.enc` 并删除临时明文；异常退出后遗留的临时数据会在下次启动时被自动回收并回写，不丢最近数据。

### 启用加密（含已有数据迁移）

1. **先备份**现有数据库文件（加密迁移前必备）：
   ```bash
   copy pennypick.db pennypick.db.bak-YYYYMMDD
   ```
2. 设置主密码环境变量并启动后端：
   ```bash
   set PENNYPICK_DB_PASS=你的强密码
   go run ./cmd/server
   ```
   首次启动检测到明文 `pennypick.db` 且无 `.enc` 时，会自动加密生成 `pennypick.db.enc`，并把原明文保留为 `pennypick.db.bak-<时间戳>` 后删除。
3. 之后每次启动都必须提供**同一个密码**：
   ```bash
   set PENNYPICK_DB_PASS=你的强密码
   go run ./cmd/server
   ```
   - 密码错误会启动失败并提示解密失败，**不会破坏数据**。
   - 未设置 `PENNYPICK_DB_PASS` 时以**明文模式**运行（仅限开发环境，启动会输出警告日志）。

> **主密码请使用强随机值并妥善保管。密码不随数据库文件保存，丢失密码将无法恢复数据。**

### 修改主密码

```bash
cd backend-go
go build -o dbencrypt.exe ./cmd/dbencrypt
dbencrypt.exe -mode chpass -file pennypick.db.enc
```

按提示输入当前密码与新密码（交互式输入、不回显；非交互环境可从 stdin 逐行读取）。

### 系统管理员如何查询数据

应用静置时磁盘上只有密文 `.enc`，管理员可通过以下方式查看数据：

**方式一：`dbview` 运维工具（推荐，深度排查）**

```bash
cd backend-go
go build -o dbview.exe ./cmd/dbview

# 1) 解密为明文 db，用 Navicat / DBeaver 打开查看
dbview.exe -file pennypick.db.enc -out view.db
# 查看完后务必删除明文文件
del view.db

# 2) 或直接导出 SQL 脚本到屏幕/文件，全程不落明文 db 文件（更安全）
dbview.exe -file pennypick.db.enc -dump > dump.sql
```

**方式二：环境变量解锁 + 应用内导出（日常运维）**

以 `PENNYPICK_DB_PASS` 环境变量启动服务后，登录系统，使用「设置 → 导出账单 CSV」即可导出一份明文 CSV 查看业务数据，无需额外工具。

**方式三：解密修改后重新加密（需要手动改数据时）**

用方式一解密得到明文 db，修改完成后用 `dbencrypt.exe -mode encrypt -in view.db -out pennypick.db.enc` 重新加密回写。

### 备份与安全建议

- **备份请备份 `.enc` 加密文件**（备份本身不含密码，安全）；同时另行妥善保管主密码。
- 明文解密产物（`view.db`、`dump.sql`）**用完即删**，不要长期留存。
- 加密参数（magic / Argon2id 参数）统一由 `internal/crypto` 管理，主程序与 `dbview` / `dbencrypt` 共用，请用同一版本代码构建，避免参数漂移导致工具无法解密。
- 系统层面可用 BitLocker / EFS 对数据盘加密，作为纵深防御的补充。

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
