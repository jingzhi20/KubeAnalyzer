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
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
  },
  content: {
    background: 'white',
    padding: '40px',
    borderRadius: '12px',
    boxShadow: '0 10px 40px rgba(0,0,0,0.1)',
    textAlign: 'center',
    maxWidth: '400px',
  },
  icon: {
    fontSize: '64px',
    marginBottom: '20px',
  },
  title: {
    fontSize: '24px',
    fontWeight: 'bold',
    marginBottom: '12px',
    color: '#333',
  },
  message: {
    fontSize: '14px',
    color: '#666',
    marginBottom: '8px',
  },
  hint: {
    fontSize: '12px',
    color: '#999',
  },
  spinner: {
    border: '4px solid #f3f3f3',
    borderTop: '4px solid #667eea',
    borderRadius: '50%',
    width: '50px',
    height: '50px',
    animation: 'spin 1s linear infinite',
    margin: '0 auto 20px',
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
