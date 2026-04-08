import { useState, useEffect } from 'react';
import { notificationApi } from '../api';
import type { NotificationConfig } from '../types';

function NotificationPage() {
  const [config, setConfig] = useState<NotificationConfig>({
    id: 0,
    webhook_url: '',
    sign_key: '',
    policy: 'anomaly_only',
    enabled: false,
    updated_at: '',
  });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await notificationApi.getConfig();
      setConfig(response.data);
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    try {
      await notificationApi.updateConfig(config);
      alert('配置已保存');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '保存失败');
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async () => {
    try {
      await notificationApi.test();
      alert('测试消息发送成功');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '测试失败');
    }
  };

  return (
    <div>
      <h1 style={styles.title}>通知配置</h1>
      
      <div style={styles.card}>
        <form autoComplete="off">
          <input type="text" name="trap-user" style={{ display: 'none' }} tabIndex={-1} autoComplete="username" />
          <input type="password" name="trap-pass" style={{ display: 'none' }} tabIndex={-1} autoComplete="current-password" />
        </form>
        <div style={styles.formGroup}>
          <label style={styles.label}>飞书 Webhook 地址</label>
          <input
            style={styles.input}
            value={config.webhook_url}
            onChange={(e) => setConfig({ ...config, webhook_url: e.target.value })}
            placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
            name="notif_webhook_url"
            autoComplete="one-time-code"
          />
        </div>

        <div style={styles.formGroup}>
          <label style={styles.label}>签名密钥（可选）</label>
          <input
            style={styles.input}
            value={config.sign_key}
            onChange={(e) => setConfig({ ...config, sign_key: e.target.value })}
            placeholder="签名密钥"
            name="notif_sign_key"
            autoComplete="one-time-code"
          />
        </div>

        <div style={styles.formGroup}>
          <label style={styles.label}>推送策略</label>
          <select
            style={styles.select}
            value={config.policy}
            onChange={(e) => setConfig({ ...config, policy: e.target.value as any })}
          >
            <option value="anomaly_only">仅在发现异常时推送</option>
            <option value="always">每次巡检均推送</option>
            <option value="disabled">关闭推送</option>
          </select>
        </div>

        <div style={styles.formGroup}>
          <label style={styles.checkbox}>
            <input
              type="checkbox"
              checked={config.enabled}
              onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
            />
            启用通知
          </label>
        </div>

        <div style={styles.actions}>
          <button style={styles.testBtn} onClick={handleTest}>发送测试消息</button>
          <button style={styles.saveBtn} onClick={handleSave} disabled={loading}>
            {loading ? '保存中...' : '保存配置'}
          </button>
        </div>
      </div>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  title: {
    fontSize: '20px',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    marginBottom: '20px',
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
    marginBottom: '6px',
    color: 'var(--k8s-text-secondary)',
    fontWeight: 500,
    fontSize: '13px',
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
  select: {
    width: '100%',
    padding: '8px 10px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '13px',
    boxSizing: 'border-box',
    outline: 'none',
  },
  checkbox: {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    fontSize: '13px',
    color: 'var(--k8s-text-secondary)',
  },
  actions: {
    display: 'flex',
    gap: '10px',
    marginTop: '24px',
  },
  testBtn: {
    padding: '8px 20px',
    background: 'var(--k8s-card-bg)',
    color: 'var(--k8s-blue)',
    border: '1px solid var(--k8s-blue)',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
  },
  saveBtn: {
    padding: '8px 20px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
};

export default NotificationPage;
