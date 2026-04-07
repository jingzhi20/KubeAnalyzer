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
        <div style={styles.formGroup}>
          <label style={styles.label}>飞书 Webhook 地址</label>
          <input
            style={styles.input}
            value={config.webhook_url}
            onChange={(e) => setConfig({ ...config, webhook_url: e.target.value })}
            placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
          />
        </div>

        <div style={styles.formGroup}>
          <label style={styles.label}>签名密钥（可选）</label>
          <input
            style={styles.input}
            value={config.sign_key}
            onChange={(e) => setConfig({ ...config, sign_key: e.target.value })}
            placeholder="签名密钥"
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
    fontSize: '28px',
    color: '#333',
    marginBottom: '24px',
  },
  card: {
    background: 'white',
    padding: '32px',
    borderRadius: '10px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
    maxWidth: '800px',
  },
  formGroup: {
    marginBottom: '24px',
  },
  label: {
    display: 'block',
    marginBottom: '8px',
    color: '#555',
    fontWeight: '500',
    fontSize: '14px',
  },
  input: {
    width: '100%',
    padding: '12px',
    border: '1px solid #ddd',
    borderRadius: '6px',
    fontSize: '14px',
    boxSizing: 'border-box',
  },
  select: {
    width: '100%',
    padding: '12px',
    border: '1px solid #ddd',
    borderRadius: '6px',
    fontSize: '14px',
    boxSizing: 'border-box',
  },
  checkbox: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    fontSize: '14px',
    color: '#555',
  },
  actions: {
    display: 'flex',
    gap: '12px',
    marginTop: '32px',
  },
  testBtn: {
    padding: '12px 24px',
    background: 'white',
    color: '#667eea',
    border: '1px solid #667eea',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
  saveBtn: {
    padding: '12px 24px',
    background: '#667eea',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
};

export default NotificationPage;
