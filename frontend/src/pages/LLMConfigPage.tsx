import { useState, useEffect } from 'react';
import { llmConfigApi } from '../api';
import type { LLMConfig } from '../types';

function LLMConfigPage() {
  const [configs, setConfigs] = useState<LLMConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    api_url: '',
    api_key: '',
    model_name: '',
  });

  useEffect(() => {
    loadConfigs();
  }, []);

  const loadConfigs = async () => {
    try {
      const response = await llmConfigApi.list();
      setConfigs(response.data);
    } catch (err) {
      console.error('Failed to load configs:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await llmConfigApi.create(formData);
      setFormData({ name: '', api_url: '', api_key: '', model_name: '' });
      setShowForm(false);
      loadConfigs();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '创建失败');
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async (id: number) => {
    try {
      await llmConfigApi.test(id);
      alert('连通性测试成功');
      loadConfigs();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '测试失败');
    }
  };

  const handleSetDefault = async (id: number) => {
    try {
      await llmConfigApi.setDefault(id);
      loadConfigs();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '设置失败');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('确定删除此配置？')) return;
    try {
      await llmConfigApi.delete(id);
      loadConfigs();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '删除失败');
    }
  };

  return (
    <div>
      <div style={styles.header}>
        <h1 style={styles.title}>LLM 配置管理</h1>
        <button style={styles.addBtn} onClick={() => setShowForm(!showForm)}>
          {showForm ? '取消' : '+ 新增配置'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} style={styles.form} autoComplete="off">
          {/* Hidden fields to trap browser autofill */}
          <input type="text" name="trap-user" style={{ display: 'none' }} tabIndex={-1} autoComplete="username" />
          <input type="password" name="trap-pass" style={{ display: 'none' }} tabIndex={-1} autoComplete="current-password" />
          <div style={styles.formGrid}>
            <div>
              <label style={styles.formLabel}>提供方名称</label>
              <input
                style={styles.input}
                placeholder="如：DeepSeek、OpenAI、通义千问"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                name="llm_provider_name"
                autoComplete="one-time-code"
                required
              />
            </div>
            <div>
              <label style={styles.formLabel}>模型名称</label>
              <input
                style={styles.input}
                placeholder="如：deepseek-chat、gpt-4o、qwen-plus"
                value={formData.model_name}
                onChange={(e) => setFormData({ ...formData, model_name: e.target.value })}
                name="llm_model_id"
                autoComplete="one-time-code"
                required
              />
            </div>
            <div>
              <label style={styles.formLabel}>API 地址（OpenAI 兼容）</label>
              <input
                style={styles.input}
                placeholder="https://api.deepseek.com"
                value={formData.api_url}
                onChange={(e) => setFormData({ ...formData, api_url: e.target.value })}
                name="llm_endpoint_url"
                autoComplete="one-time-code"
                required
              />
            </div>
            <div>
              <label style={styles.formLabel}>API Key</label>
              <input
                style={styles.input}
                placeholder="sk-..."
                type="password"
                value={formData.api_key}
                onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
                name="llm_secret_token"
                autoComplete="one-time-code"
                required
              />
            </div>
          </div>
          <button type="submit" style={styles.submitBtn} disabled={loading}>
            {loading ? '保存中...' : '保存'}
          </button>
        </form>
      )}

      <div style={styles.list}>
        {configs.map(config => (
          <div key={config.id} style={styles.card}>
            <div style={styles.cardHeader}>
              <h3 style={styles.cardTitle}>{config.name}</h3>
              <span style={{
                ...styles.badge,
                background: config.status === 'available' ? '#e8f5e9' : '#fee',
                color: config.status === 'available' ? '#4caf50' : '#f44336',
              }}>
                {config.status === 'available' ? '可用' : '不可用'}
              </span>
              {config.is_default && <span style={styles.defaultBadge}>默认</span>}
            </div>
            <div style={styles.cardBody}>
              <p style={styles.cardText}><strong>API地址：</strong>{config.api_url}</p>
              <p style={styles.cardText}><strong>模型：</strong>{config.model_name}</p>
            </div>
            <div style={styles.cardActions}>
              <button style={styles.actionBtn} onClick={() => handleTest(config.id)}>测试</button>
              {!config.is_default && (
                <button style={styles.actionBtn} onClick={() => handleSetDefault(config.id)}>设为默认</button>
              )}
              {!config.is_default && (
                <button style={{ ...styles.actionBtn, color: '#f44336' }} onClick={() => handleDelete(config.id)}>删除</button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '20px',
  },
  title: {
    fontSize: '20px',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    margin: 0,
  },
  addBtn: {
    padding: '8px 16px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  form: {
    background: 'var(--k8s-card-bg)',
    padding: '20px',
    borderRadius: 'var(--k8s-card-radius)',
    marginBottom: '20px',
    border: '1px solid var(--k8s-border)',
  },
  formGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: '12px',
    marginBottom: '14px',
  },
  input: {
    width: '100%',
    padding: '8px 10px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '13px',
    boxSizing: 'border-box' as const,
    outline: 'none',
  },
  formLabel: {
    display: 'block',
    fontSize: '12px',
    color: 'var(--k8s-text-secondary)',
    marginBottom: '4px',
    fontWeight: 500,
  },
  submitBtn: {
    width: '100%',
    padding: '10px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  list: {
    display: 'grid',
    gap: '12px',
  },
  card: {
    background: 'var(--k8s-card-bg)',
    padding: '16px 20px',
    borderRadius: 'var(--k8s-card-radius)',
    border: '1px solid var(--k8s-border)',
  },
  cardHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    marginBottom: '10px',
  },
  cardTitle: {
    fontSize: '15px',
    fontWeight: 600,
    margin: 0,
    color: 'var(--k8s-text-primary)',
  },
  badge: {
    padding: '2px 10px',
    borderRadius: '3px',
    fontSize: '11px',
    fontWeight: 500,
  },
  defaultBadge: {
    padding: '2px 10px',
    background: 'var(--k8s-blue-light)',
    color: 'var(--k8s-blue)',
    borderRadius: '3px',
    fontSize: '11px',
    fontWeight: 500,
  },
  cardBody: {
    marginBottom: '10px',
  },
  cardText: {
    fontSize: '13px',
    color: 'var(--k8s-text-secondary)',
    margin: '4px 0',
  },
  cardActions: {
    display: 'flex',
    gap: '6px',
  },
  actionBtn: {
    padding: '5px 12px',
    background: 'var(--k8s-card-bg)',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    color: 'var(--k8s-text-primary)',
  },
};

export default LLMConfigPage;
