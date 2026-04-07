import { useState, useEffect } from 'react';
import { authApi } from '../api';
import type { FeishuSSOConfig } from '../types';

function FeishuSSOConfigPage() {
  const [config, setConfig] = useState<FeishuSSOConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState({
    app_id: '',
    app_secret: '',
    redirect_uri: '',
    enabled: false,
  });
  const [showSecret, setShowSecret] = useState(false);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await authApi.getFeishuSSOConfig();
      setConfig(response.data);
      setFormData({
        app_id: response.data.app_id || '',
        app_secret: '', // Don't load secret for security
        redirect_uri: response.data.redirect_uri || '',
        enabled: response.data.enabled || false,
      });
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await authApi.updateFeishuSSOConfig(formData);
      alert('配置保存成功');
      loadConfig();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '保存失败');
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async () => {
    try {
      await authApi.testFeishuSSOConfig(formData);
      alert('配置测试成功');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '测试失败');
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h1 style={styles.title}>飞书 SSO 配置</h1>
      </div>

      <div style={styles.card}>
        <form onSubmit={handleSubmit}>
          <div style={styles.formGroup}>
            <label style={styles.label}>
              飞书应用 ID (App ID)
              <span style={styles.required}>*</span>
            </label>
            <input
              type="text"
              style={styles.input}
              placeholder="cli_xxxxxxxxxxxxx"
              value={formData.app_id}
              onChange={(e) => setFormData({ ...formData, app_id: e.target.value })}
              required
            />
            <p style={styles.hint}>
              在飞书开放平台创建应用后获取
            </p>
          </div>

          <div style={styles.formGroup}>
            <label style={styles.label}>
              飞书应用密钥 (App Secret)
              <span style={styles.required}>*</span>
            </label>
            <div style={styles.passwordWrapper}>
              <input
                type={showSecret ? 'text' : 'password'}
                style={styles.input}
                placeholder="xxxxxxxxxxxxxxxx"
                value={formData.app_secret}
                onChange={(e) => setFormData({ ...formData, app_secret: e.target.value })}
                required
              />
              <button
                type="button"
                style={styles.toggleBtn}
                onClick={() => setShowSecret(!showSecret)}
              >
                {showSecret ? '🙈' : '👁️'}
              </button>
            </div>
            <p style={styles.hint}>
              应用密钥仅在新建或修改时填写，不会回显
            </p>
          </div>

          <div style={styles.formGroup}>
            <label style={styles.label}>
              回调地址 (Redirect URI)
              <span style={styles.required}>*</span>
            </label>
            <input
              type="url"
              style={styles.input}
              placeholder="http://localhost:5173/auth/feishu/callback"
              value={formData.redirect_uri}
              onChange={(e) => setFormData({ ...formData, redirect_uri: e.target.value })}
              required
            />
            <p style={styles.hint}>
              需要在飞书开放平台的安全设置中配置相同的回调地址
            </p>
          </div>

          <div style={styles.formGroup}>
            <label style={styles.checkboxLabel}>
              <input
                type="checkbox"
                checked={formData.enabled}
                onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
              />
              <span>启用飞书 SSO 登录</span>
            </label>
            <p style={styles.hint}>
              启用后，登录页面将显示飞书登录按钮
            </p>
          </div>

          <div style={styles.actions}>
            <button
              type="button"
              style={styles.testBtn}
              onClick={handleTest}
            >
              测试配置
            </button>
            <button
              type="submit"
              style={styles.submitBtn}
              disabled={loading}
            >
              {loading ? '保存中...' : '保存配置'}
            </button>
          </div>
        </form>
      </div>

      {config?.updated_at && (
        <div style={styles.footer}>
          最后更新时间：{config.updated_at}
        </div>
      )}
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    maxWidth: '720px',
  },
  header: {
    marginBottom: '20px',
  },
  title: {
    fontSize: '20px',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    margin: 0,
  },
  card: {
    background: 'var(--k8s-card-bg)',
    padding: '24px',
    borderRadius: 'var(--k8s-card-radius)',
    border: '1px solid var(--k8s-border)',
  },
  formGroup: {
    marginBottom: '20px',
  },
  label: {
    display: 'block',
    fontSize: '13px',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    marginBottom: '6px',
  },
  required: {
    color: 'var(--k8s-danger)',
    marginLeft: '3px',
  },
  input: {
    width: '100%',
    padding: '8px 10px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '13px',
    boxSizing: 'border-box',
    outline: 'none',
  },
  passwordWrapper: {
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
  },
  toggleBtn: {
    position: 'absolute',
    right: '10px',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    fontSize: '16px',
    padding: '0',
  },
  hint: {
    fontSize: '11px',
    color: 'var(--k8s-text-muted)',
    marginTop: '4px',
    marginBottom: 0,
  },
  checkboxLabel: {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    fontSize: '13px',
    color: 'var(--k8s-text-primary)',
    cursor: 'pointer',
  },
  actions: {
    display: 'flex',
    gap: '10px',
    marginTop: '24px',
  },
  testBtn: {
    flex: 1,
    padding: '8px 20px',
    background: 'var(--k8s-card-bg)',
    border: '1px solid var(--k8s-blue)',
    color: 'var(--k8s-blue)',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  submitBtn: {
    flex: 2,
    padding: '8px 20px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  footer: {
    marginTop: '12px',
    fontSize: '11px',
    color: 'var(--k8s-text-muted)',
    textAlign: 'right',
  },
};

export default FeishuSSOConfigPage;
