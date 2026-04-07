import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { authApi } from '../api';

function FeishuCallbackPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [error, setError] = useState('');

  useEffect(() => {
    handleCallback();
  }, []);

  const handleCallback = async () => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    const savedState = sessionStorage.getItem('feishu_state');

    // Validate state
    if (!code || !state || state !== savedState) {
      setError('OAuth 状态验证失败');
      setTimeout(() => navigate('/login'), 3000);
      return;
    }

    try {
      // Exchange code for token
      const response = await authApi.feishuCallback(code);
      
      // Store token and user info
      localStorage.setItem('token', response.data.token);
      localStorage.setItem('user', JSON.stringify(response.data.user));
      
      // Clear state
      sessionStorage.removeItem('feishu_state');
      
      // Redirect to home
      navigate('/app');
    } catch (err: any) {
      setError(err.response?.data?.error?.message || '飞书登录失败');
      setTimeout(() => navigate('/login'), 3000);
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.content}>
        {error ? (
          <>
            <div style={styles.icon}>❌</div>
            <h2 style={styles.title}>登录失败</h2>
            <p style={styles.message}>{error}</p>
            <p style={styles.hint}>3秒后自动跳转到登录页...</p>
          </>
        ) : (
          <>
            <div style={styles.spinner}></div>
            <h2 style={styles.title}>正在登录...</h2>
            <p style={styles.message}>请稍候，正在完成飞书认证</p>
          </>
        )}
      </div>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: '#f7f7f7',
  },
  content: {
    background: 'white',
    padding: '36px',
    borderRadius: '6px',
    border: '1px solid #e0e0e0',
    textAlign: 'center',
    maxWidth: '380px',
  },
  icon: {
    fontSize: '48px',
    marginBottom: '16px',
  },
  title: {
    fontSize: '20px',
    fontWeight: 600,
    marginBottom: '10px',
    color: '#303030',
  },
  message: {
    fontSize: '13px',
    color: '#555',
    marginBottom: '6px',
  },
  hint: {
    fontSize: '11px',
    color: '#888',
  },
  spinner: {
    border: '3px solid #f3f3f3',
    borderTop: '3px solid #326ce5',
    borderRadius: '50%',
    width: '40px',
    height: '40px',
    animation: 'spin 1s linear infinite',
    margin: '0 auto 16px',
  },
};

// Add keyframes for spinner
const styleSheet = document.styleSheets[0];
styleSheet.insertRule(`
  @keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }
`, styleSheet.cssRules.length);

export default FeishuCallbackPage;
