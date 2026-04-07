#!/bin/bash
set -e

echo "=========================================="
echo "  KubeAnalyzer 初始化脚本"
echo "=========================================="

# ---- MySQL 配置 ----
MYSQL_HOST="localhost"
MYSQL_PORT="3306"
MYSQL_USER="root"
MYSQL_PASSWORD='Zjz5740##'
MYSQL_DATABASE="aiops"

echo ""
echo "[1/3] 创建 MySQL 数据库..."
mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "
  CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
" 2>&1

if [ $? -eq 0 ]; then
  echo "  ✅ 数据库 '$MYSQL_DATABASE' 已就绪"
else
  echo "  ❌ 数据库创建失败，请检查 MySQL 连接"
  exit 1
fi

echo ""
echo "[2/3] 生成 backend/.env 文件..."
cat > backend/.env << EOF
# MySQL
MYSQL_HOST=$MYSQL_HOST
MYSQL_PORT=$MYSQL_PORT
MYSQL_USER=$MYSQL_USER
MYSQL_PASSWORD=$MYSQL_PASSWORD
MYSQL_DATABASE=$MYSQL_DATABASE

# Server
PORT=8080

# JWT
JWT_SECRET=aiops-default-jwt-secret-key-change-in-production

# Admin (首次启动自动创建)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123
EOF
echo "  ✅ backend/.env 已生成"

echo ""
echo "[3/3] 安装前端依赖..."
if [ -d "frontend" ]; then
  (cd frontend && npm install)
  echo "  ✅ 前端依赖安装完成"
else
  echo "  ⚠️  未找到 frontend 目录，跳过"
fi

echo ""
echo "=========================================="
echo "  初始化完成"
echo "=========================================="
echo ""
echo "后续步骤:"
echo "  1. 构建后端:  cd backend && go build -o aiops-server ./cmd/main.go"
echo "  2. 启动后端:  cd backend && ./aiops-server"
echo "  3. 启动前端:  cd frontend && npm run dev"
echo "  4. 访问:      http://localhost:5173"
echo "  5. 登录:      admin / admin123"
