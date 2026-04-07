import { useState, useEffect } from 'react';
import { Outlet, useNavigate, Link, useLocation } from 'react-router-dom';
import { clusterApi } from '../api';
import type { ClusterConfig, User } from '../types';

function Layout() {
  const [collapsed, setCollapsed] = useState(false);
  const [activeCluster, setActiveCluster] = useState<ClusterConfig | null>(null);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    loadActiveCluster();
    loadCurrentUser();
  }, []);

  const loadActiveCluster = async () => {
    try {
      const response = await clusterApi.list();
      const active = response.data.find((c: ClusterConfig) => c.is_active);
      setActiveCluster(active || null);
    } catch (err) {
      console.error('Failed to load cluster:', err);
    }
  };

  const loadCurrentUser = () => {
    const userStr = localStorage.getItem('user');
    if (userStr) {
      try {
        setCurrentUser(JSON.parse(userStr));
      } catch (err) {
        console.error('Failed to parse user info:', err);
      }
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  const menuItems = [
    { path: '/app', label: '首页', icon: '🏠' },
    { path: '/app/k8sgpt', label: 'k8s 集群分析', icon: '🔍' },
    { path: '/app/kubectl-ai', label: 'kubectl-ai', icon: '🤖' },
    { path: '/app/diagnosis', label: '诊断问答', icon: '💬' },
    { path: '/app/inspections', label: '巡检管理', icon: '📊' },
    { path: '/app/clusters', label: '集群配置', icon: '☁️' },
    { path: '/app/llm-configs', label: 'LLM配置', icon: '⚙️' },
    { path: '/app/notifications', label: '通知配置', icon: '📧' },
    { path: '/app/feishu-sso', label: '飞书 SSO', icon: '🔐', adminOnly: true },
    { path: '/app/users', label: '用户管理', icon: '👥', adminOnly: true },
  ];

  // Filter menu items based on user role
  const filteredMenuItems = menuItems.filter(item => {
    if (item.adminOnly && currentUser?.role !== 'admin') {
      return false;
    }
    return true;
  });

  return (
    <div style={styles.container}>
      <aside style={{ ...styles.sidebar, width: collapsed ? '60px' : '240px' }}>
        <div style={styles.logo}>
          {!collapsed && <h2 style={{ margin: 0 }}>KubeAnalyzer</h2>}
          <button style={styles.collapseBtn} onClick={() => setCollapsed(!collapsed)}>
            {collapsed ? '→' : '←'}
          </button>
        </div>
        {!collapsed && activeCluster && (
          <div style={styles.clusterIndicator}>
            <span style={styles.clusterDot} />
            <span style={styles.clusterName}>{activeCluster.name}</span>
          </div>
        )}
        <nav style={styles.nav}>
          {filteredMenuItems.map(item => (
            <Link
              key={item.path}
              to={item.path}
              style={{
                ...styles.menuItem,
                background: (item.path === '/app' ? location.pathname === '/app' : location.pathname.startsWith(item.path)) ? '#667eea' : 'transparent',
              }}
            >
              <span style={styles.menuIcon}>{item.icon}</span>
              {!collapsed && <span>{item.label}</span>}
            </Link>
          ))}
        </nav>
        <div style={styles.userSection}>
          {!collapsed && currentUser && (
            <div style={{ position: 'relative' }}>
              <div
                style={styles.userInfo}
                onClick={() => setShowUserMenu(!showUserMenu)}
              >
                {currentUser.avatar_url ? (
                  <img src={currentUser.avatar_url} alt="avatar" style={styles.avatar} />
                ) : (
                  <div style={styles.avatarPlaceholder}>
                    {currentUser.username.charAt(0).toUpperCase()}
                  </div>
                )}
                <div style={styles.userDetails}>
                  <div style={styles.userName}>{currentUser.username}</div>
                </div>
                <span style={{
                  fontSize: '18px',
                  color: '#666',
                  transition: 'transform 0.2s',
                  transform: showUserMenu ? 'rotate(180deg)' : 'rotate(0deg)',
                  lineHeight: 1,
                }}>▾</span>
              </div>
              {showUserMenu && (
                <div style={styles.userDropdown}>
                  {currentUser.role === 'admin' && (
                    <div style={styles.dropdownRole}>管理员</div>
                  )}
                  <button style={styles.logoutBtn} onClick={handleLogout}>
                    🚪 退出登录
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </aside>
      <main style={styles.main}>
        <Outlet />
      </main>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    display: 'flex',
    minHeight: '100vh',
    background: '#f5f7fa',
  },
  sidebar: {
    background: '#fff',
    borderRight: '1px solid #e8e8e8',
    display: 'flex',
    flexDirection: 'column',
    transition: 'width 0.3s',
    overflow: 'hidden',
  },
  logo: {
    padding: '20px',
    borderBottom: '1px solid #e8e8e8',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  collapseBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    fontSize: '18px',
    padding: '5px',
  },
  nav: {
    flex: 1,
    padding: '10px 0',
  },
  clusterIndicator: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '10px 20px',
    background: '#f0f4ff',
    borderBottom: '1px solid #e8e8e8',
    fontSize: '13px',
    color: '#667eea',
  },
  clusterDot: {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#4caf50',
    display: 'inline-block',
  },
  clusterName: {
    fontWeight: '500',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  menuItem: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px 20px',
    color: '#333',
    textDecoration: 'none',
    transition: 'all 0.2s',
    marginBottom: '4px',
  },
  menuIcon: {
    fontSize: '20px',
  },
  userSection: {
    padding: '20px',
    borderTop: '1px solid #e8e8e8',
  },
  userInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px',
    background: '#f5f7fa',
    borderRadius: '8px',
    cursor: 'pointer',
    transition: 'background 0.2s',
  },
  avatar: {
    width: '40px',
    height: '40px',
    borderRadius: '50%',
    objectFit: 'cover' as const,
  },
  avatarPlaceholder: {
    width: '40px',
    height: '40px',
    borderRadius: '50%',
    background: '#667eea',
    color: 'white',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: '18px',
    fontWeight: 'bold',
  },
  userDetails: {
    flex: 1,
    overflow: 'hidden',
  },
  userName: {
    fontSize: '14px',
    fontWeight: 'bold',
    color: '#333',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  userRole: {
    fontSize: '12px',
    color: '#666',
    marginTop: '2px',
  },
  logoutBtn: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '10px 12px',
    background: 'none',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    color: '#c33',
    fontSize: '14px',
    transition: 'background 0.15s',
  },
  userDropdown: {
    marginTop: '6px',
    background: '#fff',
    borderRadius: '8px',
    boxShadow: '0 4px 16px rgba(0,0,0,0.1)',
    border: '1px solid #e8e8e8',
    padding: '6px',
    overflow: 'hidden',
  },
  dropdownRole: {
    padding: '6px 12px',
    fontSize: '12px',
    color: '#667eea',
    background: '#f0f4ff',
    borderRadius: '4px',
    marginBottom: '4px',
    textAlign: 'center' as const,
  },
  main: {
    flex: 1,
    padding: '24px',
    overflow: 'auto',
  },
};

export default Layout;
