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
              borderLeft: cluster.is_active ? '4px solid #667eea' : '4px solid transparent',
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
                  <button style={{ ...styles.actionBtn, color: '#667eea', borderColor: '#667eea' }}
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
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' },
  title: { fontSize: '28px', color: '#333', margin: 0 },
  addBtn: { padding: '10px 20px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '14px' },
  form: { background: 'white', padding: '24px', borderRadius: '10px', marginBottom: '24px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  formGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px', marginBottom: '16px' },
  input: { padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px' },
  textarea: { width: '100%', padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '13px', fontFamily: 'monospace', marginBottom: '16px', boxSizing: 'border-box' as const },
  fileUpload: { display: 'flex', alignItems: 'center' },
  fileLabel: { padding: '10px 16px', background: '#f5f5f5', border: '1px dashed #ccc', borderRadius: '6px', cursor: 'pointer', fontSize: '14px', color: '#666' },
  submitBtn: { width: '100%', padding: '12px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '14px' },
  modeBtn: { flex: 1, padding: '12px 16px', border: '2px solid #ddd', borderRadius: '8px', cursor: 'pointer', textAlign: 'center' as const, fontSize: '14px', fontWeight: '500', transition: 'all 0.2s' },
  modeBtnActive: { borderColor: '#667eea', background: '#f0f2ff', color: '#667eea' },
  list: { display: 'grid', gap: '16px' },
  empty: { textAlign: 'center' as const, padding: '60px', color: '#999', fontSize: '16px', background: 'white', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  card: { background: 'white', padding: '20px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  cardHeader: { display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '12px', flexWrap: 'wrap' as const },
  cardTitle: { fontSize: '18px', margin: 0, color: '#333' },
  badge: { padding: '4px 12px', borderRadius: '12px', fontSize: '12px', fontWeight: '500' },
  activeBadge: { padding: '4px 12px', background: '#e3f2fd', color: '#667eea', borderRadius: '12px', fontSize: '12px', fontWeight: '500' },
  cardBody: { marginBottom: '12px' },
  cardText: { fontSize: '14px', color: '#666', margin: '6px 0' },
  cardActions: { display: 'flex', gap: '8px', flexWrap: 'wrap' as const },
  actionBtn: { padding: '6px 16px', background: 'white', border: '1px solid #ddd', borderRadius: '6px', cursor: 'pointer', fontSize: '14px', color: '#333' },
  tokenPanel: { background: 'white', padding: '24px', borderRadius: '10px', marginBottom: '24px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)', border: '2px solid #1976d2' },
  codeBlock: { display: 'inline-block', padding: '4px 8px', background: '#f5f5f5', borderRadius: '4px', fontSize: '13px', fontFamily: 'monospace', wordBreak: 'break-all' as const },
  preBlock: { background: '#1e1e1e', color: '#d4d4d4', padding: '12px', borderRadius: '6px', fontSize: '13px', fontFamily: 'monospace', overflow: 'auto', whiteSpace: 'pre-wrap' as const },
};

export default ClusterPage;
