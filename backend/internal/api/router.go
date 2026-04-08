package api

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/middleware"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin router with all routes.
func SetupRouter() *gin.Engine {
	// Initialize database
	if err := database.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize inspection scheduler after database is ready
	initInspectionService()

	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Public routes (no authentication required)
	public := r.Group("/api")
	{
		// Auth routes
		auth := public.Group("/auth")
		{
			auth.POST("/login", Login)
			auth.GET("/feishu/config", GetFeishuConfig)
			auth.POST("/feishu/callback", FeishuCallback)
		}

		// Agent WebSocket endpoint (token-based auth, no JWT)
		public.GET("/agent/ws", AgentWebSocket)
	}

	// Protected routes (authentication required)
	protected := r.Group("/api")
	protected.Use(middleware.JWTAuth(), middleware.RBACAuth())
	{
		// Auth routes
		auth := protected.Group("/auth")
		{
			auth.GET("/me", GetCurrentUser)
		}

		// LLM Config routes
		llmConfigs := protected.Group("/llm-configs")
		{
			llmConfigs.GET("", ListLLMConfigs)
			llmConfigs.POST("", CreateLLMConfig)
			llmConfigs.PUT("/:id", UpdateLLMConfig)
			llmConfigs.DELETE("/:id", DeleteLLMConfig)
			llmConfigs.POST("/:id/test", TestLLMConfig)
			llmConfigs.PUT("/:id/default", SetDefaultLLMConfig)
		}

		// Diagnosis routes
		diagnosis := protected.Group("/diagnosis")
		{
			diagnosis.POST("/sessions", CreateSession)
			diagnosis.GET("/sessions", ListSessions)
			diagnosis.GET("/sessions/:id", GetSession)
			diagnosis.DELETE("/sessions/:id", DeleteSession)
			diagnosis.PUT("/sessions/:id/rename", RenameSession)
			diagnosis.POST("/sessions/:id/query", SubmitQuery)
		}

		// Inspection routes
		inspections := protected.Group("/inspections")
		{
			inspections.GET("/config", GetInspectionConfig)
			inspections.PUT("/config", UpdateInspectionConfig)
			inspections.POST("/trigger", TriggerInspection)
			inspections.GET("/tasks", GetInspectionTasks)
			inspections.GET("/tasks/:id", GetInspectionTask)
			inspections.GET("/rules", ListInspectionRules)
			inspections.POST("/rules", CreateInspectionRule)
			inspections.PUT("/rules/:id", UpdateInspectionRule)
			inspections.DELETE("/rules/:id", DeleteInspectionRule)
		}

		// Notification routes
		notifications := protected.Group("/notifications")
		{
			notifications.GET("/config", GetNotificationConfig)
			notifications.PUT("/config", UpdateNotificationConfig)
			notifications.POST("/test", TestNotification)
			notifications.POST("/send", SendNotification)
		}

		// Cluster management routes
		clusters := protected.Group("/clusters")
		{
			clusters.GET("", ListClusters)
			clusters.POST("", CreateCluster)
			clusters.PUT("/:id", UpdateCluster)
			clusters.DELETE("/:id", DeleteCluster)
			clusters.PUT("/:id/active", SetActiveCluster)
			clusters.POST("/:id/test", TestCluster)
			clusters.POST("/:id/agent-token", GenerateAgentToken)
		}

		// K8sGPT routes
		k8sgptGroup := protected.Group("/k8sgpt")
		{
			k8sgptGroup.POST("/analyze", K8sGPTAnalyze)
			k8sgptGroup.GET("/filters", K8sGPTListFilters)
			k8sgptGroup.GET("/namespaces", K8sGPTListNamespaces)
			k8sgptGroup.GET("/config", GetK8sGPTConfig)
			k8sgptGroup.PUT("/config", UpdateK8sGPTConfig)
			k8sgptGroup.POST("/config/test", TestK8sGPTConnection)
			k8sgptGroup.POST("/cache/invalidate", InvalidateK8sGPTCache)
		}

		// kubectl-ai routes
		kubectlAI := protected.Group("/kubectl-ai")
		{
			kubectlAI.POST("/generate", KubectlAIGenerate)
			kubectlAI.POST("/execute", KubectlAIExecute)
			kubectlAI.GET("/config", GetKubectlAIConfig)
			kubectlAI.PUT("/config", UpdateKubectlAIConfig)
			kubectlAI.GET("/history", GetKubectlAIHistory)
		}

		// Feishu SSO config routes (admin only, enforced by RBAC)
		feishuSSO := protected.Group("/feishu-sso")
		{
			feishuSSO.GET("/config", GetFeishuSSOConfig)
			feishuSSO.PUT("/config", UpdateFeishuSSOConfig)
			feishuSSO.POST("/test", TestFeishuSSOConfig)
		}

		// User management routes (admin only, enforced by RBAC)
		users := protected.Group("/users")
		{
			users.GET("", ListUsers)
			users.POST("", CreateUser)
			users.PUT("/:id/role", UpdateUserRole)
			users.DELETE("/:id", DeleteUser)
		}
	}

	return r
}