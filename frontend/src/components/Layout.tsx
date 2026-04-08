import { useState, useEffect, Fragment } from 'react';
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

  const [sysMenuOpen, setSysMenuOpen] = useState(false);

  type MenuItem = { path: string; label: string; icon: string; adminOnly?: boolean };
  type MenuGroup = { group: string; icon: string; children: MenuItem[] };
  type MenuEntry = MenuItem | MenuGroup;

  const isGroup = (entry: MenuEntry): entry is MenuGroup => 'group' in entry;

  const menuEntries: MenuEntry[] = [
    { path: '/app', label: '首页', icon: '🏠' },
    { path: '/app/k8sgpt', label: '集群分析', icon: '🔍' },
    { path: '/app/kubectl-ai', label: 'kubectl-ai', icon: '🤖' },
    { path: '/app/diagnosis', label: '诊断问答', icon: '💬' },
    { path: '/app/inspections', label: '巡检管理', icon: '📊' },
    { path: '/app/users', label: '用户管理', icon: '👥', adminOnly: true },
    {
      group: '系统管理',
      icon: '⚙️',
      children: [
        { path: '/app/clusters', label: '集群配置', icon: '☁️' },
        { path: '/app/llm-configs', label: 'LLM配置', icon: '🧠' },
        { path: '/app/notifications', label: '通知配置', icon: '📧' },
        { path: '/app/feishu-sso', label: '飞书 SSO', icon: '🔐', adminOnly: true },
      ],
    },
  ];

  const isActive = (item: MenuItem) =>
    item.path === '/app' ? location.pathname === '/app' : location.pathname.startsWith(item.path);

  const isAdmin = currentUser?.role === 'admin';

  // Auto-expand system menu when a child route is active
  useEffect(() => {
    const sysGroup = menuEntries.find(e => isGroup(e)) as MenuGroup | undefined;
    if (sysGroup?.children.some(c => location.pathname.startsWith(c.path))) {
      setSysMenuOpen(true);
    }
  }, [location.pathname]);

  return (
    <div style={styles.container}>
      <aside style={{ ...styles.sidebar, width: collapsed ? '56px' : '220px' }}>
        {/* Logo */}
        <div style={styles.logo}>
          {!collapsed && (
            <div style={styles.logoInner}>
              <span style={styles.logoIcon}>☸</span>
              <span style={styles.logoText}>KubeAnalyzer</span>
            </div>
          )}
          <button
            style={styles.collapseBtn}
            onClick={() => setCollapsed(!collapsed)}
            aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
          >
            {collapsed ? '›' : '‹'}
          </button>
        </div>

        {/* Cluster indicator */}
        {!collapsed && activeCluster && (
          <div style={styles.clusterIndicator}>
            <span style={styles.clusterDot} />
            <span style={styles.clusterName}>{activeCluster.name}</span>
          </div>
        )}

        {/* Nav */}
        <nav style={styles.nav}>
          {menuEntries.map((entry, idx) => {
            if (isGroup(entry)) {
              const visibleChildren = entry.children.filter(c => !c.adminOnly || isAdmin);
              if (visibleChildren.length === 0) return null;
              const groupActive = visibleChildren.some(c => location.pathname.startsWith(c.path));
              return (
                <Fragment key={idx}>
                  <div
                    style={{
                      ...styles.menuItem,
                      ...(groupActive && !sysMenuOpen ? styles.menuItemActive : {}),
                      cursor: 'pointer',
                      userSelect: 'none' as const,
                      justifyContent: collapsed ? 'center' : 'flex-start',
                    }}
                    onClick={() => setSysMenuOpen(!sysMenuOpen)}
                    role="button"
                    tabIndex={0}
                    aria-expanded={sysMenuOpen}
                    onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') setSysMenuOpen(!sysMenuOpen); }}
                  >
                    <span style={styles.menuIcon}>{entry.icon}</span>
                    {!collapsed && (
                      <>
                        <span style={{ ...styles.menuLabel, flex: 1 }}>{entry.group}</span>
                        <span style={{
                          fontSize: '10px',
                          color: '#888',
                          transition: 'transform 0.2s',
                          transform: sysMenuOpen ? 'rotate(180deg)' : 'rotate(0deg)',
                        }}>▾</span>
                      </>
                    )}
                  </div>
                  {sysMenuOpen && visibleChildren.map(child => (
                    <Link
                      key={child.path}
                      to={child.path}
                      style={{
                        ...styles.menuItem,
                        ...(isActive(child) ? styles.menuItemActive : {}),
                        paddingLeft: collapsed ? '16px' : '32px',
                      }}
                    >
                      <span style={styles.menuIcon}>{child.icon}</span>
                      {!collapsed && <span style={styles.menuLabel}>{child.label}</span>}
                    </Link>
                  ))}
                </Fragment>
              );
            }
            const item = entry as MenuItem;
            if (item.adminOnly && !isAdmin) return null;
            return (
              <Link
                key={item.path}
                to={item.path}
                style={{
                  ...styles.menuItem,
                  ...(isActive(item) ? styles.menuItemActive : {}),
                }}
              >
                <span style={styles.menuIcon}>{item.icon}</span>
                {!collapsed && <span style={styles.menuLabel}>{item.label}</span>}
              </Link>
            );
          })}
        </nav>

        {/* User section */}
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
                  fontSize: '14px',
                  color: '#999',
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
                    退出登录
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
    background: 'var(--k8s-bg)',
  },
  sidebar: {
    background: 'var(--k8s-sidebar-bg)',
    display: 'flex',
    flexDirection: 'column',
    transition: 'width 0.2s ease',
    overflow: 'hidden',
    flexShrink: 0,
  },
  logo: {
    padding: '16px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    borderBottom: '1px solid rgba(255,255,255,0.08)',
    minHeight: '56px',
  },
  logoInner: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
  },
  logoIcon: {
    fontSize: '22px',
    color: '#326ce5',
  },
  logoText: {
    fontSize: '16px',
    fontWeight: 600,
    color: '#fff',
    letterSpacing: '-0.3px',
    whiteSpace: 'nowrap' as const,
  },
  collapseBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    fontSize: '18px',
    padding: '4px 6px',
    color: '#999',
    borderRadius: '4px',
    lineHeight: 1,
  },
  clusterIndicator: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '8px 16px',
    background: 'rgba(50,108,229,0.1)',
    borderBottom: '1px solid rgba(255,255,255,0.05)',
    fontSize: '12px',
    color: '#7aafff',
  },
  clusterDot: {
    width: '6px',
    height: '6px',
    borderRadius: '50%',
    background: '#4caf50',
    display: 'inline-block',
    flexShrink: 0,
  },
  clusterName: {
    fontWeight: 500,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  nav: {
    flex: 1,
    padding: '8px 0',
    overflowY: 'auto',
  },
  menuItem: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    padding: '9px 16px',
    color: '#bbb',
    textDecoration: 'none',
    transition: 'all 0.15s',
    fontSize: '13px',
    borderLeft: '3px solid transparent',
    whiteSpace: 'nowrap' as const,
  },
  menuItemActive: {
    color: '#fff',
    background: 'rgba(50,108,229,0.2)',
    borderLeftColor: '#326ce5',
  },
  menuIcon: {
    fontSize: '16px',
    width: '20px',
    textAlign: 'center' as const,
    flexShrink: 0,
  },
  menuLabel: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  userSection: {
    padding: '12px 16px',
    borderTop: '1px solid rgba(255,255,255,0.08)',
  },
  userInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    padding: '8px 10px',
    background: 'rgba(255,255,255,0.06)',
    borderRadius: '6px',
    cursor: 'pointer',
    transition: 'background 0.15s',
  },
  avatar: {
    width: '32px',
    height: '32px',
    borderRadius: '50%',
    objectFit: 'cover' as const,
  },
  avatarPlaceholder: {
    width: '32px',
    height: '32px',
    borderRadius: '50%',
    background: '#326ce5',
    color: 'white',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: '14px',
    fontWeight: 600,
    flexShrink: 0,
  },
  userDetails: {
    flex: 1,
    overflow: 'hidden',
  },
  userName: {
    fontSize: '13px',
    fontWeight: 500,
    color: '#ddd',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  logoutBtn: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    padding: '8px 10px',
    background: 'none',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    color: '#ef5350',
    fontSize: '13px',
    transition: 'background 0.15s',
  },
  userDropdown: {
    marginTop: '6px',
    background: '#2a2a2a',
    borderRadius: '6px',
    border: '1px solid rgba(255,255,255,0.1)',
    padding: '4px',
    overflow: 'hidden',
  },
  dropdownRole: {
    padding: '4px 10px',
    fontSize: '11px',
    color: '#7aafff',
    background: 'rgba(50,108,229,0.15)',
    borderRadius: '4px',
    marginBottom: '4px',
    textAlign: 'center' as const,
  },
  main: {
    flex: 1,
    padding: '24px 32px',
    overflow: 'auto',
    maxHeight: '100vh',
  },
};

export default Layout;
