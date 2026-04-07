import api from './client';
import type { LoginRequest, LoginResponse, User, LLMConfig, DiagnosisSession, DiagnosisRecord, InspectionConfig, InspectionRule, InspectionTask, NotificationConfig, ClusterConfig, K8sGPTConfig, K8sGPTAnalyzeResponse, KubectlAIConfig, KubectlAIResult, KubectlAIHistory, FeishuConfig, FeishuSSOConfig, CreateUserRequest, UpdateRoleRequest, UserInfo } from '../types';

export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  getMe: () => api.get<User>('/auth/me'),
  getFeishuConfig: () => api.get<FeishuConfig>('/auth/feishu/config'),
  feishuCallback: (code: string) => api.post<LoginResponse>('/auth/feishu/callback', { code }),
  getFeishuSSOConfig: () => api.get<FeishuSSOConfig>('/feishu-sso/config'),
  updateFeishuSSOConfig: (data: { app_id: string; app_secret: string; redirect_uri: string; enabled: boolean }) => 
    api.put<FeishuSSOConfig>('/feishu-sso/config', data),
  testFeishuSSOConfig: (data: { app_id: string; app_secret: string; redirect_uri: string; enabled: boolean }) => 
    api.post('/feishu-sso/test', data),
};

export const llmConfigApi = {
  list: () => api.get<LLMConfig[]>('/llm-configs'),
  create: (data: Partial<LLMConfig>) => api.post<LLMConfig>('/llm-configs', data),
  update: (id: number, data: Partial<LLMConfig>) => api.put<LLMConfig>(`/llm-configs/${id}`, data),
  delete: (id: number) => api.delete(`/llm-configs/${id}`),
  test: (id: number) => api.post(`/llm-configs/${id}/test`),
  setDefault: (id: number) => api.put(`/llm-configs/${id}/default`),
};

export const diagnosisApi = {
  createSession: (data: { title: string }) => api.post<DiagnosisSession>('/diagnosis/sessions', data),
  listSessions: () => api.get<DiagnosisSession[]>('/diagnosis/sessions'),
  getSession: (id: number) => api.get<DiagnosisSession>(`/diagnosis/sessions/${id}`),
  submitQuery: (sessionId: number, data: { question: string }) => 
    api.post<DiagnosisRecord>(`/diagnosis/sessions/${sessionId}/query`, data),
};

export const inspectionApi = {
  getConfig: () => api.get<InspectionConfig>('/inspections/config'),
  updateConfig: (data: { cron_expression: string; enabled: boolean }) => 
    api.put<InspectionConfig>('/inspections/config', data),
  trigger: (clusterID?: number) => api.post<InspectionTask>('/inspections/trigger', clusterID ? { cluster_id: clusterID } : {}),
  listTasks: () => api.get<InspectionTask[]>('/inspections/tasks'),
  getTask: (id: number) => api.get<InspectionTask>(`/inspections/tasks/${id}`),
  listRules: () => api.get<InspectionRule[]>('/inspections/rules'),
  createRule: (data: Partial<InspectionRule>) => 
    api.post<InspectionRule>('/inspections/rules', data),
  updateRule: (id: number, data: Partial<InspectionRule>) =>
    api.put<InspectionRule>(`/inspections/rules/${id}`, data),
  deleteRule: (id: number) => api.delete(`/inspections/rules/${id}`),
};

export const notificationApi = {
  getConfig: () => api.get<NotificationConfig>('/notifications/config'),
  updateConfig: (data: Partial<NotificationConfig>) => api.put<NotificationConfig>('/notifications/config', data),
  test: () => api.post('/notifications/test'),
  send: (data: { title: string; summary: string; details: string[] }) => 
    api.post('/notifications/send', data),
};

export const clusterApi = {
  list: () => api.get<ClusterConfig[]>('/clusters'),
  create: (data: { name: string; kubeconfig?: string; context?: string; server_url?: string; conn_mode?: string }) =>
    api.post<ClusterConfig>('/clusters', data),
  update: (id: number, data: { name: string; kubeconfig?: string; context?: string; server_url?: string; conn_mode?: string }) =>
    api.put<ClusterConfig>(`/clusters/${id}`, data),
  delete: (id: number) => api.delete(`/clusters/${id}`),
  setActive: (id: number) => api.put(`/clusters/${id}/active`),
  test: (id: number) => api.post(`/clusters/${id}/test`),
  generateAgentToken: (id: number) => api.post<{ token: string; ws_url: string; deploy_cmd: string }>(`/clusters/${id}/agent-token`),
};

export const k8sgptApi = {
  analyze: (data: { filters?: string[]; namespace?: string; label_selector?: string; explain?: boolean; with_stats?: boolean; use_cache?: boolean; cluster_id?: number }) =>
    api.post<K8sGPTAnalyzeResponse>('/k8sgpt/analyze', data),
  listFilters: () => api.get<{ filters: string[] }>('/k8sgpt/filters'),
  listNamespaces: (clusterId?: number) => api.get<{ namespaces: string[] }>('/k8sgpt/namespaces', { params: clusterId ? { cluster_id: clusterId } : undefined }),
  getConfig: () => api.get<K8sGPTConfig>('/k8sgpt/config'),
  updateConfig: (data: Partial<K8sGPTConfig>) => api.put<K8sGPTConfig>('/k8sgpt/config', data),
  testConnection: (clusterId?: number) => api.post('/k8sgpt/config/test', clusterId ? { cluster_id: clusterId } : {}),
  invalidateCache: () => api.post('/k8sgpt/cache/invalidate'),
};

export const kubectlAiApi = {
  generate: (data: { prompt: string }) => api.post<KubectlAIResult>('/kubectl-ai/generate', data),
  execute: (data: { prompt: string }) => api.post<KubectlAIResult>('/kubectl-ai/execute', data),
  getConfig: () => api.get<KubectlAIConfig>('/kubectl-ai/config'),
  updateConfig: (data: Partial<KubectlAIConfig>) => api.put<KubectlAIConfig>('/kubectl-ai/config', data),
  getHistory: () => api.get<KubectlAIHistory[]>('/kubectl-ai/history'),
};

export const userApi = {
  list: () => api.get<UserInfo[]>('/users'),
  create: (data: CreateUserRequest) => api.post<UserInfo>('/users', data),
  updateRole: (id: number, data: UpdateRoleRequest) => api.put(`/users/${id}/role`, data),
  delete: (id: number) => api.delete(`/users/${id}`),
};
