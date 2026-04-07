import { useState, useEffect } from 'react';
import { clusterApi } from '../api';
import type { ClusterConfig } from '../types';

function ClusterPage() {
  const [clusters, setClusters] = useState<ClusterConfig[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [loading, setLoading] = useState(false);
  const [testingId, setTestingId] = useState<number | null>(null);
  const [agentTokenInfo, setAgentTokenInfo] = useState<{ id: number; token: string; ws_url: string; deploy_cmd: string } | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    kubeconfig: '',
    context: '',
    server_url: '',
    conn_mode: 'direct' as 'direct' | 'agent',
  });

  useEffect(() => {
    loadClusters();
  }, []);

  const loadClusters = async () => {
    try {
      const response = await clusterApi.list();
      setClusters(response.data);
    } catch (err) {
      console.error('Failed to load clusters:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await clusterApi.create(formData);
      setFormData({ name: '', kubeconfig: '', context: '', server_url: '', conn_mode: 'direct' });
      setShowForm(false);
      loadClusters();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '创建失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSetActive = async (id: number) => {
    try {
      await clusterApi.setActive(id);
      loadClusters();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '切换失败');
    }
  };

  const handleTest = async (id: number) => {
    setTestingId(id);
    try {
      await clusterApi.test(id);
      alert('集群连通性测试成功');
      loadClusters();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '测试失败');
      loadClusters();
    } finally {
      setTestingId(null);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('确定删除此集群配置？')) return;
    try {
      await clusterApi.delete(id);
      loadClusters();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '删除失败');
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      setFormData({ ...formData, kubeconfig: event.target?.result as string });
    };
    reader.readAsText(file);
  };

  const handleGenerateToken = async (id: number) => {
    try {
      const resp = await clusterApi.generateAgentToken(id);
      setAgentTokenInfo({ id, ...resp.data });
      loadClusters();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '生成 Token 失败');
    }
  };

  const statusColor = (status: string) => {
    switch (status) {
      case 'connected': return { bg: '#e8f5e9', color: '#4caf50' };
      case 'disconnected': return { bg: '#fee', color: '#f44336' };
      default: return { bg: '#fff3cd', color: '#ff9800' };
    }
  };

  const statusText = (status: string) => {
    switch (status) {
      case 'connected': return '已连接';
      case 'disconnected': return '未连接';
      default: return '未知';
    }
  };

  const agentStatusColor = (status: string) => {
    return status === 'online'
      ? { bg: '#e8f5e9', color: '#4caf50' }
      : { bg: '#fee', color: '#f44336' };
  };

  const connModeLabel = (mode: string) => {
    return mode === 'agent' ? 'Agent' : '直连';
  };

  return (
    <div>
      <div style={styles.header}>
        <h1 style={styles.title}>集群管理</h1>
        <button style={styles.addBtn} onClick={() => setShowForm(!showForm)}>
          {showForm ? '取消' : '+ 添加集群'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} style={styles.form}>
          {/* Connection mode selector */}
          <div style={{ display: 'flex', gap: '12px', marginBottom: '16px' }}>
            <label style={{
              ...styles.modeBtn,
              ...(formData.conn_mode === 'direct' ? styles.modeBtnActive : {}),
            }}>
              <input
                type="radio" name="conn_mode" value="direct"
                checked={formData.conn_mode === 'direct'}
                onChange={() => setFormData({ ...formData, conn_mode: 'direct' })}
                style={{ display: 'none' }}
              />
              直连模式
              <span style={{ fontSize: '12px', color: '#888', display: 'block' }}>
                通过 kubeconfig 直接连接
              </span>
            </label>
            <label style={{
              ...styles.modeBtn,
              ...(formData.conn_mode === 'agent' ? styles.modeBtnActive : {}),
            }}>
              <input
                type="radio" name="conn_mode" value="agent"
                checked={formData.conn_mode === 'agent'}
                onChange={() => setFormData({ ...formData, conn_mode: 'agent' })}
                style={{ display: 'none' }}
              />
              Agent 模式
              <span style={{ fontSize: '12px', color: '#888', display: 'block' }}>
                远程集群部署 Agent 反连
              </span>
            </label>
          </div>

          <div style={styles.formGrid}>
            <input
              style={styles.input}
              placeholder="集群名称"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
            />
            <input
              style={styles.input}
              placeholder="Server URL（可选）"
              value={formData.server_url}
              onChange={(e) => setFormData({ ...formData, server_url: e.target.value })}
            />
            {formData.conn_mode === 'direct' && (
              <>
                <input
                  style={styles.input}
                  placeholder="Context（可选）"
                  value={formData.context}
                  onChange={(e) => setFormData({ ...formData, context: e.target.value })}
                />
                <div style={styles.fileUpload}>
                  <label style={styles.fileLabel}>
                    上传 kubeconfig 文件
                    <input type="file" accept=".yaml,.yml,.conf,*" onChange={handleFileUpload} style={{ display: 'none' }} />
                  </label>
                </div>
              </>
            )}
          </div>

          {formData.conn_mode === 'direct' && (
            <textarea
              style={styles.textarea}
              placeholder="粘贴 kubeconfig 内容（或通过上方上传文件）"
              value={formData.kubeconfig}
              onChange={(e) => setFormData({ ...formData, kubeconfig: e.target.value })}
              rows={8}
              required
            />
          )}

          {formData.conn_mode === 'agent' && (
            <div style={{ padding: '16px', background: '#f8f9fa', borderRadius: '8px', marginBottom: '16px', fontSize: '14px', color: '#555' }}>
              Agent 模式无需提供 kubeconfig。创建集群后，点击"生成 Agent Token"获取部署命令，
              在远程集群中部署 AIOps Agent 即可自动连接。
            </div>
          )}

          <button type="submit" style={styles.submitBtn} disabled={loading}>
            {loading ? '保存中...' : '保存集群'}
          </button>
        </form>
      )}

      {/* Agent Token Info Modal */}
      {agentTokenInfo && (
        <div style={styles.tokenPanel}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <h3 style={{ margin: 0, fontSize: '16px' }}>Agent 部署信息</h3>
            <button style={{ ...styles.actionBtn, padding: '4px 12px' }} onClick={() => setAgentTokenInfo(null)}>关闭</button>
          </div>
          <div style={{ marginBottom: '8px' }}>
            <strong>Token：</strong>
            <code style={styles.codeBlock}>{agentTokenInfo.token}</code>
          </div>
          <div style={{ marginBottom: '8px' }}>
            <strong>WebSocket URL：</strong>
            <code style={styles.codeBlock}>{agentTokenInfo.ws_url}</code>
          </div>
          <div>
            <strong>部署命令：</strong>
            <pre style={styles.preBlock}>{agentTokenInfo.deploy_cmd}</pre>
          </div>
          <button
            style={{ ...styles.submitBtn, marginTop: '8px' }}
            onClick={() => {
              navigator.clipboard.writeText(agentTokenInfo.deploy_cmd);
              alert('已复制到剪贴板');
            }}
          >
            一键复制部署命令
          </button>
        </div>
      )}

      <div style={styles.list}>
        {clusters.length === 0 && (
          <div style={styles.empty}>暂无集群配置，请添加一个 K8s 集群</div>
        )}
        {clusters.map(cluster => {
          const sc = statusColor(cluster.status);
          const ac = agentStatusColor(cluster.agent_status);
          return (
            <div key={cluster.id} style={{
              ...styles.card,
              borderLeft: cluster.is_active ? '3px solid var(--k8s-blue)' : '3px solid transparent',
            }}>
              <div style={styles.cardHeader}>
                <h3 style={styles.cardTitle}>{cluster.name}</h3>
                <span style={{
                  ...styles.badge,
                  background: cluster.conn_mode === 'agent' ? '#e3f2fd' : '#f3e5f5',
                  color: cluster.conn_mode === 'agent' ? '#1976d2' : '#7b1fa2',
                }}>
                  {connModeLabel(cluster.conn_mode)}
                </span>
                <span style={{ ...styles.badge, background: sc.bg, color: sc.color }}>
                  {statusText(cluster.status)}
                </span>
                {cluster.conn_mode === 'agent' && (
                  <span style={{ ...styles.badge, background: ac.bg, color: ac.color }}>
                    Agent {cluster.agent_status === 'online' ? '在线' : '离线'}
                  </span>
                )}
                {cluster.is_active && <span style={styles.activeBadge}>当前活跃</span>}
              </div>
              <div style={styles.cardBody}>
                {cluster.context && <p style={styles.cardText}><strong>Context：</strong>{cluster.context}</p>}
                {cluster.server_url && <p style={styles.cardText}><strong>Server：</strong>{cluster.server_url}</p>}
                <p style={styles.cardText}><strong>创建时间：</strong>{new Date(cluster.created_at).toLocaleString()}</p>
                {cluster.conn_mode === 'agent' && cluster.last_ping_at && (
                  <p style={styles.cardText}><strong>最后心跳：</strong>{new Date(cluster.last_ping_at).toLocaleString()}</p>
                )}
                {cluster.conn_mode === 'agent' && cluster.agent_status === 'online' && (
                  <p style={styles.cardText}>
                    <strong>安全策略：</strong>
                    <span style={{
                      padding: '2px 8px',
                      borderRadius: '4px',
                      fontSize: '12px',
                      fontWeight: 500,
                      background: cluster.allow_write ? '#fff3e0' : '#e8f5e9',
                      color: cluster.allow_write ? '#e65100' : '#2e7d32',
                    }}>
                      {cluster.allow_write ? '写权限已开启' : '只读模式'}
                    </span>
                    <span style={{ fontSize: '12px', color: '#999', marginLeft: '8px' }}>
                      {cluster.allow_write
                        ? '允许 delete Pod/Deployment、scale、rollout 等；禁止删除 Namespace/Node/CRD 等高危操作'
                        : '仅允许 get/describe/logs 等只读操作'}
                    </span>
                  </p>
                )}
              </div>
              <div style={styles.cardActions}>
                <button
                  style={styles.actionBtn}
                  onClick={() => handleTest(cluster.id)}
                  disabled={testingId === cluster.id}
                >
                  {testingId === cluster.id ? '测试中...' : '测试连通性'}
                </button>
                {cluster.conn_mode === 'agent' && (
                  <button
                    style={{ ...styles.actionBtn, color: '#1976d2', borderColor: '#1976d2' }}
                    onClick={() => handleGenerateToken(cluster.id)}
                  >
                    生成 Agent Token
                  </button>
                )}
                {!cluster.is_active && (
                  <button style={{ ...styles.actionBtn, color: 'var(--k8s-blue)', borderColor: 'var(--k8s-blue)' }}
                    onClick={() => handleSetActive(cluster.id)}>
                    设为活跃
                  </button>
                )}
                {!cluster.is_active && (
                  <button style={{ ...styles.actionBtn, color: '#f44336' }}
                    onClick={() => handleDelete(cluster.id)}>
                    删除
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' },
  title: { fontSize: '20px', fontWeight: 600, color: 'var(--k8s-text-primary)', margin: 0 },
  addBtn: { padding: '8px 16px', background: 'var(--k8s-blue)', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', fontWeight: 500 },
  form: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: 'var(--k8s-card-radius)', marginBottom: '20px', border: '1px solid var(--k8s-border)' },
  formGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '12px', marginBottom: '14px' },
  input: { padding: '8px 10px', border: '1px solid var(--k8s-border)', borderRadius: '4px', fontSize: '13px', outline: 'none' },
  textarea: { width: '100%', padding: '8px 10px', border: '1px solid var(--k8s-border)', borderRadius: '4px', fontSize: '12px', fontFamily: 'var(--k8s-mono)', marginBottom: '14px', boxSizing: 'border-box' as const },
  fileUpload: { display: 'flex', alignItems: 'center' },
  fileLabel: { padding: '8px 14px', background: '#f5f5f5', border: '1px dashed var(--k8s-border)', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', color: 'var(--k8s-text-secondary)' },
  submitBtn: { width: '100%', padding: '10px', background: 'var(--k8s-blue)', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', fontWeight: 500 },
  modeBtn: { flex: 1, padding: '10px 14px', border: '1px solid var(--k8s-border)', borderRadius: '4px', cursor: 'pointer', textAlign: 'center' as const, fontSize: '13px', fontWeight: 500, transition: 'all 0.15s', background: 'var(--k8s-card-bg)' },
  modeBtnActive: { borderColor: 'var(--k8s-blue)', background: 'var(--k8s-blue-light)', color: 'var(--k8s-blue)' },
  list: { display: 'grid', gap: '12px' },
  empty: { textAlign: 'center' as const, padding: '48px', color: 'var(--k8s-text-muted)', fontSize: '14px', background: 'var(--k8s-card-bg)', borderRadius: 'var(--k8s-card-radius)', border: '1px solid var(--k8s-border)' },
  card: { background: 'var(--k8s-card-bg)', padding: '16px 20px', borderRadius: 'var(--k8s-card-radius)', border: '1px solid var(--k8s-border)' },
  cardHeader: { display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '10px', flexWrap: 'wrap' as const },
  cardTitle: { fontSize: '15px', fontWeight: 600, margin: 0, color: 'var(--k8s-text-primary)' },
  badge: { padding: '2px 10px', borderRadius: '3px', fontSize: '11px', fontWeight: 500 },
  activeBadge: { padding: '2px 10px', background: 'var(--k8s-blue-light)', color: 'var(--k8s-blue)', borderRadius: '3px', fontSize: '11px', fontWeight: 500 },
  cardBody: { marginBottom: '10px' },
  cardText: { fontSize: '13px', color: 'var(--k8s-text-secondary)', margin: '4px 0' },
  cardActions: { display: 'flex', gap: '6px', flexWrap: 'wrap' as const },
  actionBtn: { padding: '5px 12px', background: 'var(--k8s-card-bg)', border: '1px solid var(--k8s-border)', borderRadius: '4px', cursor: 'pointer', fontSize: '12px', color: 'var(--k8s-text-primary)', transition: 'border-color 0.15s' },
  tokenPanel: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: 'var(--k8s-card-radius)', marginBottom: '20px', border: '2px solid var(--k8s-blue)' },
  codeBlock: { display: 'inline-block', padding: '3px 6px', background: '#f5f5f5', borderRadius: '3px', fontSize: '12px', fontFamily: 'var(--k8s-mono)', wordBreak: 'break-all' as const },
  preBlock: { background: '#1e1e1e', color: '#d4d4d4', padding: '12px', borderRadius: '4px', fontSize: '12px', fontFamily: 'var(--k8s-mono)', overflow: 'auto', whiteSpace: 'pre-wrap' as const },
};

export default ClusterPage;
