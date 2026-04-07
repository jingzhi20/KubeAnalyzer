import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api';
import AnimatedCharacters from '../components/login/AnimatedCharacters';
import './LoginPage.css';

function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [isTyping, setIsTyping] = useState(false);
  const [loginFailed, setLoginFailed] = useState(false);
  const [loginSuccess, setLoginSuccess] = useState(false);
  const [feishuEnabled, setFeishuEnabled] = useState(false);
  const [feishuAppId, setFeishuAppId] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    loadFeishuConfig();
  }, []);

  const loadFeishuConfig = async () => {
    try {
      const response = await authApi.getFeishuConfig();
      setFeishuEnabled(response.data.enabled);
      setFeishuAppId(response.data.app_id);
    } catch (err) {
      console.error('Failed to load Feishu config:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    setLoginFailed(false);
    setLoginSuccess(false);

    try {
      const response = await authApi.login({ username, password });
      localStorage.setItem('token', response.data.token);
      localStorage.setItem('user', JSON.stringify(response.data.user));
      setLoginSuccess(true);
      
      // 登录成功后延迟跳转，让用户看到庆祝动画
      setTimeout(() => {
        navigate('/app');
      }, 2000);
    } catch (err: any) {
      setError(err.response?.data?.error?.message || '登录失败，请重试');
      setLoginFailed(true);
      
      // 3秒后重置失败状态
      setTimeout(() => {
        setLoginFailed(false);
      }, 3000);
    } finally {
      setLoading(false);
    }
  };

  const handleFeishuLogin = () => {
    // Generate random state and store in sessionStorage
    const state = Math.random().toString(36).substring(2);
    sessionStorage.setItem('feishu_state', state);
    
    // Construct Feishu OAuth URL
    const redirectURI = encodeURIComponent(window.location.origin + '/auth/feishu/callback');
    const feishuAuthURL = `https://open.feishu.cn/open-apis/authen/v1/authorize?app_id=${feishuAppId}&redirect_uri=${redirectURI}&state=${state}&response_type=code`;
    
    // Redirect to Feishu
    window.location.href = feishuAuthURL;
  };

  return (
    <div className="login-page">
      {/* Left Content Section with Animated Characters */}
      <div className="left-section">
        <div className="characters-section">
          <AnimatedCharacters
            isTyping={isTyping}
            showPassword={showPassword}
            passwordLength={password.length}
            loginFailed={loginFailed}
            loginSuccess={loginSuccess}
          />
        </div>

        {/* Decorative elements */}
        <div className="grid-overlay"></div>
        <div className="blur-circle blur-circle-1"></div>
        <div className="blur-circle blur-circle-2"></div>
      </div>

      {/* Right Login Section */}
      <div className="right-section">
        <div className="form-wrapper">
          {/* Mobile Logo */}
          <div className="mobile-logo">
            <div className="logo-image">🤖</div>
            <span>AIOps 智能诊断平台</span>
          </div>

          {/* Header */}
          <div className="form-header">
            <h1 className="form-title">KubeAnalyzer</h1>
          </div>

          {/* Login Form */}
          <form onSubmit={handleSubmit} className="login-form" autoComplete="off">
            {/* Username Field */}
            <div className="form-group">
              <label htmlFor="username" className="form-label">用户名</label>
              <input
                id="username"
                type="text"
                placeholder="请输入用户名"
                className="form-input"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                onFocus={() => setIsTyping(true)}
                onBlur={() => setIsTyping(false)}
                autoComplete="off"
                required
                spellCheck="false"
              />
            </div>

            {/* Password Field */}
            <div className="form-group">
              <label htmlFor="password" className="form-label">密码</label>
              <div className="password-wrapper">
                <input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  placeholder="请输入密码"
                  className="form-input"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="off"
                  spellCheck="false"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="password-toggle"
                >
                  {showPassword ? (
                    <svg className="icon" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/>
                      <circle cx="12" cy="12" r="3"/>
                    </svg>
                  ) : (
                    <svg className="icon" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/>
                      <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/>
                      <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/>
                      <line x1="2" x2="22" y1="2" y2="22"/>
                    </svg>
                  )}
                </button>
              </div>
            </div>

            {/* Error Alert */}
            {error && (
              <div className="error-alert">
                {error}
              </div>
            )}

            {/* Submit Button */}
            <button
              type="submit"
              className="submit-button"
              disabled={loading}
            >
              <span className="button-text">{loading ? '登录中...' : '登录'}</span>
              <svg className="button-icon" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M5 12h14"/>
                <path d="m12 5 7 7-7 7"/>
              </svg>
            </button>

            {/* Feishu SSO Button */}
            {feishuEnabled && (
              <>
                <div className="divider">
                  <span>或</span>
                </div>
                <button
                  type="button"
                  className="feishu-button"
                  onClick={handleFeishuLogin}
                >
                  <svg className="feishu-icon" viewBox="0 0 24 24" width="20" height="20">
                    <path fill="currentColor" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
                  </svg>
                  <span>飞书登录</span>
                </button>
              </>
            )}
          </form>
        </div>
      </div>
    </div>
  );
}

export default LoginPage;
