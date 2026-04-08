package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user.
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:100;not null;uniqueIndex:uni_users_username" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	DisplayName  string    `gorm:"size:100" json:"display_name"`
	Role         string    `gorm:"size:20;not null;default:user" json:"role"`
	FeishuOpenID string    `gorm:"size:100;uniqueIndex:uni_users_feishu_open_id" json:"-"`
	AvatarURL    string    `gorm:"size:500" json:"avatar_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// LLMConfig represents a LLM provider configuration.
type LLMConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	APIURL    string    `gorm:"size:500;not null" json:"api_url"`
	APIKey    string    `gorm:"size:500;not null" json:"api_key"`
	ModelName string    `gorm:"size:100;not null" json:"model_name"`
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	Status    string    `gorm:"size:20;default:unavailable" json:"status"` // available / unavailable
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DiagnosisSession represents a diagnostic session.
type DiagnosisSession struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	UserID    uint              `gorm:"not null;index" json:"user_id"`
	Title     string            `gorm:"size:500" json:"title"`
	Records   []DiagnosisRecord `gorm:"foreignKey:SessionID" json:"records,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// DiagnosisRecord represents a single diagnosis Q&A record.
type DiagnosisRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SessionID    uint      `gorm:"not null;index" json:"session_id"`
	Question     string    `gorm:"type:text;not null" json:"question"`
	GRPCResponse string    `gorm:"type:text" json:"grpc_response"`
	LLMResponse  string    `gorm:"type:text" json:"llm_response"`
	LLMAvailable bool      `gorm:"default:true" json:"llm_available"`
	CreatedAt    time.Time `json:"created_at"`
}

// InspectionConfig represents the inspection scheduling configuration.
type InspectionConfig struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CronExpression string    `gorm:"size:100;not null" json:"cron_expression"`
	Enabled        bool      `gorm:"default:false" json:"enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// InspectionRule represents an inspection rule for specific resource types.
type InspectionRule struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"size:200;not null" json:"name"`
	RuleType       string    `gorm:"size:20;not null;default:builtin" json:"rule_type"` // builtin / command / script
	IsDefault      bool      `gorm:"default:false" json:"is_default"`                   // true = system preset rule
	ResourceType   string    `gorm:"size:50" json:"resource_type"`
	CheckItems     string    `gorm:"type:text" json:"check_items"`
	Command        string    `gorm:"type:text" json:"command"`
	Script         string    `gorm:"type:text" json:"script"`
	ScriptType     string    `gorm:"size:20" json:"script_type"`
	Timeout        int       `gorm:"default:60" json:"timeout"`
	TargetNodes    string    `gorm:"type:text" json:"target_nodes"`
	Namespaces     string    `gorm:"type:text" json:"namespaces"`
	ClusterID      uint      `gorm:"default:0" json:"cluster_id"`        // 0 = active cluster
	CronExpression string    `gorm:"size:100" json:"cron_expression"`    // per-rule cron schedule
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// InspectionTask represents a single inspection task execution.
type InspectionTask struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TriggerType   string    `gorm:"size:20;not null" json:"trigger_type"` // scheduled / manual
	Status        string    `gorm:"size:20;not null" json:"status"`       // running / completed / failed
	ClusterID     uint      `gorm:"default:0" json:"cluster_id"`
	ClusterName   string    `gorm:"size:100" json:"cluster_name"`
	AnomalyCount  int       `gorm:"default:0" json:"anomaly_count"`
	GRPCResponse  string    `gorm:"type:text" json:"grpc_response"`
	ScriptResults string    `gorm:"type:text" json:"script_results"`      // JSON array of script execution results
	LLMSummary    string    `gorm:"type:text" json:"llm_summary"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

// NotificationConfig represents the Feishu webhook notification configuration.
type NotificationConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	WebhookURL string   `gorm:"size:500;not null" json:"webhook_url"`
	SignKey   string    `gorm:"size:500" json:"-"` // encrypted storage
	Policy    string    `gorm:"size:20;default:anomaly_only" json:"policy"` // anomaly_only / always / disabled
	Enabled   bool      `gorm:"default:false" json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ClusterConfig represents a Kubernetes cluster configuration.
type ClusterConfig struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:100;not null" json:"name"`
	KubeConfig     string     `gorm:"type:text" json:"kubeconfig" binding:"-"` // required for direct mode, optional for agent mode
	Context        string     `gorm:"size:200" json:"context"`
	IsActive       bool       `gorm:"default:false" json:"is_active"`
	Status         string     `gorm:"size:20;default:unknown" json:"status"`    // connected / disconnected / unknown
	ServerURL      string     `gorm:"size:500" json:"server_url"`
	ConnMode       string     `gorm:"size:20;default:direct" json:"conn_mode"` // direct / agent
	AgentToken     string     `gorm:"size:500" json:"-"`                       // Agent authentication token
	AgentStatus    string     `gorm:"size:20;default:offline" json:"agent_status"` // online / offline
	AllowWrite     bool       `gorm:"default:false" json:"allow_write"`        // Agent 是否允许写操作
	LastPingAt     *time.Time `json:"last_ping_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// K8sGPTConfig represents the built-in cluster analysis configuration.
type K8sGPTConfig struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Backend        string    `gorm:"size:50;default:openai" json:"backend"` // openai / azure / localai / ollama etc.
	Model          string    `gorm:"size:100" json:"model"`
	BaseURL        string    `gorm:"size:500" json:"base_url"`
	Language       string    `gorm:"size:20;default:chinese" json:"language"`
	Anonymize      bool      `gorm:"default:false" json:"anonymize"`
	UseBuiltinLLM  bool      `gorm:"default:true" json:"use_builtin_llm"` // true = use system default LLM
	UpdatedAt      time.Time `json:"updated_at"`
}

// KubectlAIConfig represents the built-in kubectl AI assistant configuration.
type KubectlAIConfig struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Temperature    float64   `gorm:"default:0.7" json:"temperature"`
	SystemPrompt   string    `gorm:"type:text" json:"system_prompt"`
	CustomExamples string    `gorm:"type:text" json:"custom_examples"`       // JSON array of example strings
	UpdatedAt      time.Time `json:"updated_at"`
}

// KubectlAIHistory represents a kubectl-ai command history record.
type KubectlAIHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	ClusterID uint      `gorm:"index" json:"cluster_id"`
	Prompt    string    `gorm:"type:text;not null" json:"prompt"`
	Command   string    `gorm:"type:text" json:"command"`
	Output    string    `gorm:"type:text" json:"output"`
	Executed  bool      `gorm:"default:false" json:"executed"`
	CreatedAt time.Time `json:"created_at"`
}

// FeishuSSOConfig represents the Feishu SSO configuration.
type FeishuSSOConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       string    `gorm:"size:200" json:"app_id"`
	AppSecret   string    `gorm:"size:500" json:"-"` // encrypted storage
	RedirectURI string    `gorm:"size:500" json:"redirect_uri"`
	Enabled     bool      `gorm:"default:false" json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// fixLegacyIndexes drops legacy unique indexes on the users table that may
// conflict with GORM AutoMigrate. Uses raw SQL to avoid GORM's migrator
// incorrectly issuing DROP FOREIGN KEY instead of DROP INDEX.
func fixLegacyIndexes(db *gorm.DB) {
	if !db.Migrator().HasTable("users") {
		return
	}

	legacy := []string{
		"uni_users_username",
		"uni_users_feishu_open_id",
	}
	for _, name := range legacy {
		// Check if the index actually exists in information_schema.
		var count int64
		db.Raw(
			"SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = ?",
			name,
		).Scan(&count)
		if count > 0 {
			db.Exec("ALTER TABLE `users` DROP INDEX `" + name + "`")
		}
	}
}

// InitModels performs auto migration for all models.
func InitModels(db *gorm.DB) error {
	// Fix legacy index names before AutoMigrate to avoid "Can't DROP" errors.
	fixLegacyIndexes(db)

	return db.AutoMigrate(
		&User{},
		&LLMConfig{},
		&DiagnosisSession{},
		&DiagnosisRecord{},
		&InspectionConfig{},
		&InspectionRule{},
		&InspectionTask{},
		&NotificationConfig{},
		&ClusterConfig{},
		&K8sGPTConfig{},
		&KubectlAIConfig{},
		&KubectlAIHistory{},
		&FeishuSSOConfig{},
	)
}
