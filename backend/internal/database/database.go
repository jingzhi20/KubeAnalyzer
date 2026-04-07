package database

import (
	"fmt"
	"log"
	"os"

	"aiops-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Init initializes the MySQL database and performs auto migration.
func Init() error {
	// Read MySQL configuration from environment variables
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("MYSQL_PORT")
	if port == "" {
		port = "3306"
	}

	user := os.Getenv("MYSQL_USER")
	if user == "" {
		user = "root"
	}

	password := os.Getenv("MYSQL_PASSWORD")
	database := os.Getenv("MYSQL_DATABASE")
	if database == "" {
		database = "aiops"
	}

	// Configure MySQL connection
	config := mysqlDriver.Config{
		User:                 user,
		Passwd:               password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%s", host, port),
		DBName:               database,
		ParseTime:            true,
		Loc:                  nil,
		AllowNativePasswords: true,
	}

	var err error
	DB, err = gorm.Open(mysql.Open(config.FormatDSN()), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL database: %w", err)
	}

	log.Printf("Connected to MySQL database: %s@%s:%s/%s", user, host, port, database)

	// Auto migrate all models
	if err := model.InitModels(DB); err != nil {
		return err
	}

	// Create default admin user if not exists
	if err := createDefaultAdmin(); err != nil {
		return err
	}

	// Seed default inspection rules
	if err := seedDefaultInspectionRules(); err != nil {
		log.Printf("Warning: failed to seed default inspection rules: %v", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

// createDefaultAdmin creates a default admin user if no users exist.
func createDefaultAdmin() error {
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		return nil
	}

	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := model.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         "admin",
		DisplayName:  "管理员",
	}

	if err := DB.Create(&admin).Error; err != nil {
		return err
	}

	log.Printf("Default admin user created: %s", username)
	return nil
}

// seedDefaultInspectionRules creates default inspection rules if none exist.
func seedDefaultInspectionRules() error {
	var count int64
	DB.Model(&model.InspectionRule{}).Where("is_default = ?", true).Count(&count)
	if count > 0 {
		return nil
	}

	rules := []model.InspectionRule{
		seedRuleBusinessLogs(),
		seedRuleNetworkComponents(),
		seedRuleNodeResources(),
		seedRuleNodeHeartbeat(),
		seedRulePodStatus(),
	}

	for _, rule := range rules {
		if err := DB.Create(&rule).Error; err != nil {
			return err
		}
		log.Printf("Seeded default inspection rule: %s", rule.Name)
	}
	return nil
}

func seedRuleBusinessLogs() model.InspectionRule {
	return model.InspectionRule{
		Name:       "检查业务服务异常日志（HTTP 3xx/4xx/500）",
		RuleType:   "script",
		IsDefault:  true,
		ScriptType: "bash",
		Timeout:    120,
		Namespaces: "lingcast",
		Enabled:    true,
		Script: `#!/bin/bash
# 环境变量 INSPECTION_NAMESPACES: 逗号分隔的 namespace，为空则默认 lingcast
SERVICES=("apiserver" "user-center")
ALERT_LINES=""

if [ -n "$INSPECTION_NAMESPACES" ]; then
  IFS=',' read -ra NS_LIST <<< "$INSPECTION_NAMESPACES"
else
  NS_LIST=("lingcast")
fi

for NS in "${NS_LIST[@]}"; do
  NS=$(echo "$NS" | xargs)
  echo "=============================="
  echo "命名空间: $NS"
  echo "=============================="
  for SVC in "${SERVICES[@]}"; do
    PODS=$(kubectl get pods -n "$NS" --no-headers -o custom-columns=":metadata.name" 2>/dev/null | grep "$SVC" || true)
    for POD in $PODS; do
      [ -z "$POD" ] && continue
      echo "--- Pod: $POD ---"
      ERRORS=$(kubectl logs --tail=500 -n "$NS" "$POD" 2>/dev/null | \
        grep -iE '"status":\s*(3[0-9]{2}|4[0-9]{2}|500)|HTTP/[0-9.]+ (3[0-9]{2}|4[0-9]{2}|500)|\b(3[0-9]{2}|4[0-9]{2}|500)\b.*\b(GET|POST|PUT|DELETE|PATCH)\b' || true)
      if [ -n "$ERRORS" ]; then
        COUNT=$(echo "$ERRORS" | wc -l | tr -d ' ')
        echo "[预警] $NS/$POD 发现 $COUNT 条异常HTTP状态码日志"
        echo "$ERRORS" | tail -20
        ALERT_LINES="$ALERT_LINES\n[预警] $NS/$POD: ${COUNT}条异常"
      else
        echo "[正常] 未发现异常HTTP状态码"
      fi
    done
  done
done

if [ -n "$ALERT_LINES" ]; then
  echo ""
  echo "========== 预警汇总 =========="
  echo -e "$ALERT_LINES"
  exit 1
fi
echo "所有业务服务日志检查正常"
exit 0
`,
	}
}

func seedRuleNetworkComponents() model.InspectionRule {
	return model.InspectionRule{
		Name:       "检查集群网络组件状态",
		RuleType:   "script",
		IsDefault:  true,
		ScriptType: "bash",
		Timeout:    120,
		Enabled:    true,
		Script: `#!/bin/bash
# 跨所有 namespace 自动发现并检查网络组件
# 覆盖: calico, flannel, coredns, kube-proxy, nginx-ingress, traefik, cilium, metallb, istio, envoy 等
NETWORK_KEYWORDS="calico|flannel|coredns|kube-proxy|nginx-ingress|ingress-nginx|traefik|cilium|metallb|kube-dns|envoy|istio"
HAS_ALERT=0
ALERT_LINES=""

echo "=== 集群网络组件巡检 ==="
echo ""

# 自动发现所有 namespace 中的网络组件 Pod
echo "--- 自动发现网络组件 ---"
ALL_NET_PODS=$(kubectl get pods --all-namespaces --no-headers \
  -o custom-columns="NS:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount,NODE:.spec.nodeName" \
  2>/dev/null | grep -iE "$NETWORK_KEYWORDS" || true)

if [ -z "$ALL_NET_PODS" ]; then
  echo "[警告] 未发现任何网络组件 Pod"
  exit 0
fi

echo "$ALL_NET_PODS"
echo ""

# 检查非 Running 状态
echo "--- 异常状态检查 ---"
ABNORMAL=$(echo "$ALL_NET_PODS" | awk '$3 != "Running" && $3 != "Succeeded" {print $0}' || true)
if [ -n "$ABNORMAL" ]; then
  echo "[预警] 发现非 Running 状态的网络组件:"
  echo "$ABNORMAL"
  ALERT_LINES="$ALERT_LINES\n[预警] 网络组件异常状态:\n$ABNORMAL"
  HAS_ALERT=1
else
  echo "[正常] 所有网络组件均为 Running 状态"
fi

# 检查重启次数 > 5
echo ""
echo "--- 重启次数检查 ---"
HIGH_RESTART=$(echo "$ALL_NET_PODS" | awk '$4 != "<none>" && $4+0 > 5 {print $0}' || true)
if [ -n "$HIGH_RESTART" ]; then
  echo "[预警] 以下网络组件重启次数超过 5 次:"
  echo "$HIGH_RESTART"
  ALERT_LINES="$ALERT_LINES\n[预警] 网络组件频繁重启:\n$HIGH_RESTART"
  HAS_ALERT=1
else
  echo "[正常] 无频繁重启的网络组件"
fi

# 检查错误日志
echo ""
echo "--- 错误日志检查 ---"
CHECKED=0
while IFS= read -r LINE; do
  [ -z "$LINE" ] && continue
  NS=$(echo "$LINE" | awk '{print $1}')
  POD=$(echo "$LINE" | awk '{print $2}')
  CHECKED=$((CHECKED + 1))
  ERRORS=$(kubectl logs --tail=100 -n "$NS" "$POD" 2>/dev/null | \
    grep -iE "error|fail|timeout|refused|unreachable|connection reset" | \
    grep -viE "no error|error=nil|level=info" | tail -5 || true)
  if [ -n "$ERRORS" ]; then
    COUNT=$(echo "$ERRORS" | wc -l | tr -d ' ')
    echo "[预警] $NS/$POD 发现 $COUNT 条异常日志:"
    echo "$ERRORS"
    ALERT_LINES="$ALERT_LINES\n[预警] $NS/$POD: ${COUNT}条错误日志"
    HAS_ALERT=1
  fi
done <<< "$ALL_NET_PODS"

echo ""
echo "共检查 $CHECKED 个网络组件 Pod"

if [ "$HAS_ALERT" -eq 1 ]; then
  echo ""
  echo "========== 预警汇总 =========="
  echo -e "$ALERT_LINES"
  exit 1
fi
echo "所有网络组件运行正常"
exit 0
`,
	}
}

func seedRuleNodeResources() model.InspectionRule {
	return model.InspectionRule{
		Name:        "检查节点资源使用率",
		RuleType:    "script",
		IsDefault:   true,
		ScriptType:  "bash",
		Timeout:     120,
		TargetNodes: "",
		Enabled:     true,
		Script: `#!/bin/bash
# 检查节点 CPU、内存、磁盘使用率，超过 80% 阈值预警
# 环境变量 INSPECTION_TARGET_NODES: 逗号分隔的节点名，为空则检查所有节点
THRESHOLD=80
HAS_ALERT=0
ALERT_LINES=""

if [ -n "$INSPECTION_TARGET_NODES" ]; then
  IFS=',' read -ra NODES <<< "$INSPECTION_TARGET_NODES"
else
  mapfile -t NODES < <(kubectl get nodes --no-headers -o custom-columns=":metadata.name" 2>/dev/null)
fi

if [ ${#NODES[@]} -eq 0 ]; then
  echo "[错误] 未找到任何节点"
  exit 1
fi

echo "=== 节点资源使用率检查 ==="
echo "阈值: ${THRESHOLD}%  |  目标节点: ${NODES[*]}"
echo ""

for NODE in "${NODES[@]}"; do
  NODE=$(echo "$NODE" | xargs)
  echo "=============================="
  echo "节点: $NODE"
  echo "=============================="

  NODE_STATUS=$(kubectl get node "$NODE" --no-headers 2>/dev/null || true)
  if [ -z "$NODE_STATUS" ]; then
    echo "[错误] 节点 $NODE 不存在或无法访问"
    ALERT_LINES="$ALERT_LINES\n[错误] 节点 $NODE 不可达"
    HAS_ALERT=1
    continue
  fi

  # CPU / 内存（需要 metrics-server）
  TOP_OUTPUT=$(kubectl top node "$NODE" --no-headers 2>/dev/null || true)
  if [ -n "$TOP_OUTPUT" ]; then
    CPU_PCT=$(echo "$TOP_OUTPUT" | awk '{gsub(/%/,""); print $3}')
    MEM_PCT=$(echo "$TOP_OUTPUT" | awk '{gsub(/%/,""); print $5}')
    echo "CPU: ${CPU_PCT}%  |  内存: ${MEM_PCT}%"
    if [ "$(echo "$CPU_PCT > $THRESHOLD" | bc -l 2>/dev/null || echo 0)" = "1" ]; then
      echo "[预警] CPU ${CPU_PCT}% 超过阈值"
      ALERT_LINES="$ALERT_LINES\n[预警] $NODE CPU: ${CPU_PCT}%"
      HAS_ALERT=1
    fi
    if [ "$(echo "$MEM_PCT > $THRESHOLD" | bc -l 2>/dev/null || echo 0)" = "1" ]; then
      echo "[预警] 内存 ${MEM_PCT}% 超过阈值"
      ALERT_LINES="$ALERT_LINES\n[预警] $NODE 内存: ${MEM_PCT}%"
      HAS_ALERT=1
    fi
  else
    echo "[警告] 无法获取 metrics 数据"
  fi

  # 磁盘（通过 node stats API）
  DISK_JSON=$(kubectl get --raw "/api/v1/nodes/$NODE/proxy/stats/summary" 2>/dev/null || true)
  if [ -n "$DISK_JSON" ]; then
    AVAIL=$(echo "$DISK_JSON" | grep -o '"availableBytes":[0-9]*' | head -1 | cut -d: -f2)
    CAP=$(echo "$DISK_JSON" | grep -o '"capacityBytes":[0-9]*' | head -1 | cut -d: -f2)
    if [ -n "$AVAIL" ] && [ -n "$CAP" ] && [ "$CAP" -gt 0 ]; then
      USED=$((CAP - AVAIL))
      DISK_PCT=$((USED * 100 / CAP))
      echo "磁盘: ${DISK_PCT}% ($((USED/1024/1024/1024))GB / $((CAP/1024/1024/1024))GB)"
      if [ "$DISK_PCT" -gt "$THRESHOLD" ]; then
        echo "[预警] 磁盘 ${DISK_PCT}% 超过阈值"
        ALERT_LINES="$ALERT_LINES\n[预警] $NODE 磁盘: ${DISK_PCT}%"
        HAS_ALERT=1
      fi
    fi
  else
    echo "[警告] 无法获取磁盘数据"
  fi

  # 节点 Conditions
  CONDITIONS=$(kubectl get node "$NODE" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null || true)
  for COND in DiskPressure MemoryPressure PIDPressure; do
    if echo "$CONDITIONS" | grep -q "${COND}=True"; then
      echo "[预警] 节点存在 ${COND}"
      ALERT_LINES="$ALERT_LINES\n[预警] $NODE ${COND}"
      HAS_ALERT=1
    fi
  done
  echo ""
done

if [ "$HAS_ALERT" -eq 1 ]; then
  echo "========== 预警汇总 =========="
  echo -e "$ALERT_LINES"
  exit 1
fi
echo "所有节点资源使用率正常"
exit 0
`,
	}
}

func seedRuleNodeHeartbeat() model.InspectionRule {
	return model.InspectionRule{
		Name:        "检查节点时间同步与心跳状态",
		RuleType:    "script",
		IsDefault:   true,
		ScriptType:  "bash",
		Timeout:     60,
		TargetNodes: "",
		Enabled:     true,
		Script: `#!/bin/bash
# 检查节点心跳，超过 5 分钟未更新则预警
# 环境变量 INSPECTION_TARGET_NODES: 逗号分隔的节点名，为空则检查所有节点
HAS_ALERT=0
ALERT_LINES=""

if [ -n "$INSPECTION_TARGET_NODES" ]; then
  IFS=',' read -ra NODES <<< "$INSPECTION_TARGET_NODES"
else
  mapfile -t NODES < <(kubectl get nodes --no-headers -o custom-columns=":metadata.name" 2>/dev/null)
fi

echo "=== 节点时间同步检查 ==="
echo "目标节点: ${NODES[*]}"
echo ""

for NODE in "${NODES[@]}"; do
  NODE=$(echo "$NODE" | xargs)
  echo "--- $NODE ---"

  NODE_JSON=$(kubectl get node "$NODE" -o json 2>/dev/null || true)
  if [ -z "$NODE_JSON" ]; then
    echo "[错误] 无法获取节点信息"
    ALERT_LINES="$ALERT_LINES\n[错误] $NODE 不可达"
    HAS_ALERT=1
    continue
  fi

  OS_IMG=$(echo "$NODE_JSON" | grep -o '"osImage":"[^"]*"' | cut -d'"' -f4)
  KV=$(echo "$NODE_JSON" | grep -o '"kubeletVersion":"[^"]*"' | cut -d'"' -f4)
  echo "系统: $OS_IMG | Kubelet: $KV"

  HB=$(kubectl get node "$NODE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastHeartbeatTime}' 2>/dev/null || true)
  if [ -n "$HB" ]; then
    echo "最近心跳: $HB"
    HB_TS=$(date -d "$HB" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%SZ" "$HB" +%s 2>/dev/null || echo 0)
    NOW_TS=$(date +%s)
    if [ "$HB_TS" -gt 0 ]; then
      DIFF=$((NOW_TS - HB_TS))
      if [ "$DIFF" -gt 300 ]; then
        echo "[预警] 心跳延迟 ${DIFF}s，可能存在时间同步问题"
        ALERT_LINES="$ALERT_LINES\n[预警] $NODE 心跳延迟 ${DIFF}s"
        HAS_ALERT=1
      else
        echo "[正常] 心跳延迟 ${DIFF}s"
      fi
    fi
  fi

  READY=$(kubectl get node "$NODE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)
  if [ "$READY" != "True" ]; then
    echo "[预警] 节点 Ready 状态: $READY"
    ALERT_LINES="$ALERT_LINES\n[预警] $NODE NotReady"
    HAS_ALERT=1
  fi
  echo ""
done

if [ "$HAS_ALERT" -eq 1 ]; then
  echo "========== 预警汇总 =========="
  echo -e "$ALERT_LINES"
  exit 1
fi
echo "所有节点时间同步正常"
exit 0
`,
	}
}

func seedRulePodStatus() model.InspectionRule {
	return model.InspectionRule{
		Name:       "检查 lingcast 服务 Pod 状态",
		RuleType:   "script",
		IsDefault:  true,
		ScriptType: "bash",
		Timeout:    60,
		Namespaces: "lingcast",
		Enabled:    true,
		Script: `#!/bin/bash
# 环境变量 INSPECTION_NAMESPACES: 逗号分隔的 namespace，为空则默认 lingcast
HAS_ALERT=0
ALERT_LINES=""

if [ -n "$INSPECTION_NAMESPACES" ]; then
  IFS=',' read -ra NS_LIST <<< "$INSPECTION_NAMESPACES"
else
  NS_LIST=("lingcast")
fi

for NS in "${NS_LIST[@]}"; do
  NS=$(echo "$NS" | xargs)
  echo "=== $NS namespace Pod 状态 ==="
  kubectl get pods -n "$NS" -o wide 2>/dev/null || echo "[警告] 无法获取 $NS 下的 Pod"
  echo ""
  ABNORMAL=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | \
    awk '$3 != "Running" && $3 != "Completed" && $3 != "Succeeded" {print $0}' || true)
  if [ -n "$ABNORMAL" ]; then
    echo "[预警] $NS 发现异常 Pod:"
    echo "$ABNORMAL"
    COUNT=$(echo "$ABNORMAL" | wc -l | tr -d ' ')
    ALERT_LINES="$ALERT_LINES\n[预警] $NS: ${COUNT}个异常Pod"
    HAS_ALERT=1
  else
    echo "[正常] $NS 所有 Pod 运行正常"
  fi
  echo ""
done

if [ "$HAS_ALERT" -eq 1 ]; then
  echo "========== 预警汇总 =========="
  echo -e "$ALERT_LINES"
  exit 1
fi
echo "所有服务 Pod 状态正常"
exit 0
`,
	}
}
