# KubeAnalyzer 本地启动指南

## 环境要求

| 依赖 | 版本 |
|------|------|
| Go | 1.19+ |
| Node.js | 18+ |
| MySQL | 8.0+ |

## 快速开始

### 1. 初始化（创建数据库 + 安装依赖）

```bash
bash scripts/init.sh
```

脚本会自动完成：
- 创建 MySQL 数据库 `aiops`（字符集 utf8mb4）
- 生成 `backend/.env` 配置文件
- 安装前端 npm 依赖

### 2. 构建并启动后端

```bash
cd backend
go build -o aiops-server ./cmd/main.go
./aiops-server
```

后端启动后会自动完成数据库表迁移和默认数据初始化。

服务地址：http://localhost:8080

### 3. 启动前端

```bash
cd frontend
npm run dev
```

访问地址：http://localhost:5173

### 4. 登录

默认管理员账号：
- 用户名：`admin`
- 密码：`admin123`

## 环境变量说明（backend/.env）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| MYSQL_HOST | localhost | MySQL 地址 |
| MYSQL_PORT | 3306 | MySQL 端口 |
| MYSQL_USER | root | MySQL 用户 |
| MYSQL_PASSWORD | - | MySQL 密码 |
| MYSQL_DATABASE | aiops | 数据库名 |
| PORT | 8080 | 后端端口 |
| JWT_SECRET | (内置默认值) | JWT 签名密钥 |
| ADMIN_USERNAME | admin | 初始管理员用户名 |
| ADMIN_PASSWORD | admin123 | 初始管理员密码 |

## 项目结构

```
├── agent/          # K8s 集群 Agent（部署在目标集群）
├── backend/        # Go 后端（Gin + GORM + MySQL）
├── frontend/       # React 前端（Vite + TypeScript）
└── scripts/        # 初始化脚本
```
