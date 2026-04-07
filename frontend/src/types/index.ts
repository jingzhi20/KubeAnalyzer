export interface User {
  id: number;
  username: string;
  display_name: string;
  role: 'admin' | 'user';
  avatar_url: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  expires_at: number;
  user: User;
}

export interface LLMConfig {
  id: number;
  name: string;
  api_url: string;
  api_key: string;
  model_name: string;
  is_default: boolean;
  status: 'available' | 'unavailable';
  created_at: string;
  updated_at: string;
}

export interface DiagnosisSession {
  id: number;
  user_id: number;
  title: string;
  records?: DiagnosisRecord[];
  created_at: string;
}

export interface DiagnosisRecord {
  id: number;
  session_id: number;
  question: string;
  grpc_response: string;
  llm_response: string;
  llm_available: boolean;
  created_at: string;
}

export interface InspectionConfig {
  id: number;
  cron_expression: string;
  enabled: boolean;
  updated_at: string;
}

export interface InspectionRule {
  id: number;
  name: string;
  rule_type: 'builtin' | 'command' | 'script';
  is_default: boolean;
  resource_type: string;
  check_items: string;
  command: string;
  script: string;
  script_type: 'bash' | 'python' | 'perl';
  timeout: number;
  target_nodes: string;
  namespaces: string;
  cluster_id: number;
  cron_expression: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface InspectionTask {
  id: number;
  trigger_type: 'scheduled' | 'manual';
  status: 'running' | 'completed' | 'failed';
  cluster_id: number;
  cluster_name: string;
  anomaly_count: number;
  grpc_response: string;
  script_results: string;
  llm_summary: string;
  error_message?: string;
  started_at: string;
  completed_at?: string;
}

export interface ScriptResult {
  rule_id: number;
  rule_name: string;
  rule_type: string;
  exit_code: number;
  stdout: string;
  stderr: string;
  duration: string;
  error?: string;
}

export interface NotificationConfig {
  id: number;
  webhook_url: string;
  sign_key: string;
  policy: 'anomaly_only' | 'always' | 'disabled';
  enabled: boolean;
  updated_at: string;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    suggestion?: string;
    timestamp: string;
  };
}

export interface ClusterConfig {
  id: number;
  name: string;
  kubeconfig: string;
  context: string;
  is_active: boolean;
  status: 'connected' | 'disconnected' | 'unknown';
  server_url: string;
  conn_mode: 'direct' | 'agent';
  agent_status: 'online' | 'offline';
  allow_write: boolean;  // Agent 是否允许写操作
  last_ping_at?: string;
  created_at: string;
  updated_at: string;
}

export interface K8sGPTConfig {
  id: number;
  backend: string;
  model: string;
  base_url: string;
  language: string;
  anonymize: boolean;
  use_builtin_llm: boolean;
  updated_at: string;
}

export interface K8sGPTAnalyzeResult {
  kind: string;
  name: string;
  error: string[];
  details: string;
  parentObject: string;
}

export interface K8sGPTAnalyzeStats {
  analyzer: string;
  duration: string;
}

export interface K8sGPTAnalyzeResponse {
  provider: string;
  errors: number;
  status: string;
  problems: number;
  results: K8sGPTAnalyzeResult[];
  stats?: K8sGPTAnalyzeStats[];
  raw_json?: string;
}

export interface KubectlAIConfig {
  id: number;
  temperature: number;
  system_prompt: string;
  custom_examples: string;  // JSON array
  updated_at: string;
}

export interface KubectlAIResult {
  prompt: string;
  command: string;
  output?: string;
}

export interface KubectlAIHistory {
  id: number;
  user_id: number;
  cluster_id: number;
  prompt: string;
  command: string;
  output: string;
  executed: boolean;
  created_at: string;
}

export interface FeishuConfig {
  enabled: boolean;
  app_id: string;
}

export interface FeishuSSOConfig {
  id: number;
  app_id: string;
  redirect_uri: string;
  enabled: boolean;
  updated_at: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  display_name: string;
  role: 'admin' | 'user';
}

export interface UpdateRoleRequest {
  role: 'admin' | 'user';
}

export interface UserInfo {
  id: number;
  username: string;
  display_name: string;
  role: 'admin' | 'user';
  avatar_url: string;
  login_method: string;
  created_at: string;
}
