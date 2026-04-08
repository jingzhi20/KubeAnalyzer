import { useState, useEffect } from 'react';
import { Outlet, useNavigate, Link, useLocation } from 'react-router-dom';
import { clusterApi } from '../api';
import type { ClusterConfig, User } from '../types';
import {
  Home,
  Search,
  Bot,
  MessageCircle,
  ClipboardCheck,
  Users,
  Server,
  Brain,
  Bell,
  KeyRound,
  Settings,
  ChevronDown,
  LogOut,
  LayoutDashboard,
} from 'lucide-react';

type MenuItem = {
  path: string;
  label: string;
  icon: React.ReactNode;
  adminOnly?: boolean;
  gradient?: string;        // pastel gradient for active state
  iconActive?: string;      // icon color when active
};

type MenuGroup = {
  group: string;
  icon: React.ReactNode;
  children: MenuItem[];
};

type MenuEntry = MenuItem | MenuGroup;

const isGroup = (entry: MenuEntry): entry is MenuGroup => 'group' in entry;

function Layout() {
  const [activeCluster, setActiveCluster] = useState<ClusterConfig | null>(null);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const [sysMenuOpen, setSysMenuOpen] = useState(false);
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

  /* ── Pastel gradient palette for each menu item ── */
  const mainMenu: MenuItem[] = [
    {
      path: '/app', label: 'Overview', icon: <Home size={18} />,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
    {
      path: '/app/k8sgpt', label: '快速诊断', icon: <Search size={18} />,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
    {
      path: '/app/kubectl-ai', label: 'kubectl-ai', icon: <Bot size={18} />,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
    {
      path: '/app/diagnosis', label: '多轮问答', icon: <MessageCircle size={18} />,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
    {
      path: '/app/inspections', label: '巡检管理', icon: <ClipboardCheck size={18} />,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
    {
      path: '/app/users', label: '用户管理', icon: <Users size={18} />, adminOnly: true,
      gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
      iconActive: 'text-cyan-600',
    },
  ];

  const sysGroup: MenuGroup = {
    group: '系统管理',
    icon: <Settings size={18} />,
    children: [
      {
        path: '/app/clusters', label: '集群配置', icon: <Server size={18} />,
        gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
        iconActive: 'text-cyan-600',
      },
      {
        path: '/app/llm-configs', label: 'LLM配置', icon: <Brain size={18} />,
        gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
        iconActive: 'text-cyan-600',
      },
      {
        path: '/app/notifications', label: '通知配置', icon: <Bell size={18} />,
        gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
        iconActive: 'text-cyan-600',
      },
      {
        path: '/app/feishu-sso', label: '飞书 SSO', icon: <KeyRound size={18} />, adminOnly: true,
        gradient: 'bg-gradient-to-r from-cyan-100/90 to-blue-50/70',
        iconActive: 'text-cyan-600',
      },
    ],
  };


  const menuEntries: MenuEntry[] = [...mainMenu, sysGroup];

  const isActive = (item: MenuItem) =>
    item.path === '/app'
      ? location.pathname === '/app'
      : location.pathname.startsWith(item.path);

  const isAdmin = currentUser?.role === 'admin';

  useEffect(() => {
    if (sysGroup.children.some((c) => location.pathname.startsWith(c.path))) {
      setSysMenuOpen(true);
    }
  }, [location.pathname]);

  return (
    <div className="flex h-screen bg-[#0a0a0a] p-2 gap-0 font-sans selection:bg-indigo-100 selection:text-indigo-900">
      {/* ── Sidebar ── */}
      <aside className="w-[260px] flex-shrink-0 bg-white/95 backdrop-blur-2xl rounded-[1.5rem] shadow-[0_8px_30px_rgb(0,0,0,0.04)] ring-1 ring-gray-100/50 flex flex-col overflow-hidden mr-3">

        {/* Logo */}
        <div style={{ padding: '40px 20px 24px 20px' }} className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-2xl bg-gradient-to-br from-gray-800 to-gray-900 shadow-md shadow-gray-900/20 flex items-center justify-center border border-gray-700/50">
              <LayoutDashboard size={18} className="text-white" />
            </div>
            <span className="font-bold text-gray-800 text-[17px] tracking-tight">KubeAnalyzer</span>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 overflow-y-auto pb-6 scrollbar-hide flex flex-col">
          {/* Main Menu Group */}
          <div className="flex-1 flex flex-col">
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }} className="flex-1">
              {menuEntries.map((entry, idx) => {
                if (isGroup(entry)) {
                  const visibleChildren = entry.children.filter(
                    (c) => !c.adminOnly || isAdmin
                  );
                  if (visibleChildren.length === 0) return null;
                  return (
                    <div key={idx} className="mt-6 text-gray-900">
                      <button
                        onClick={() => setSysMenuOpen(!sysMenuOpen)}
                        style={{
                          padding: '14px 16px',
                          borderRadius: '16px',
                          boxShadow: '0 1px 4px rgba(0,0,0,0.04), 0 0 0 1px rgba(0,0,0,0.03)',
                        }}
                        className={`w-full flex items-center gap-3 text-[14px] font-medium transition-all duration-300 ${
                          sysMenuOpen
                            ? 'text-gray-900 bg-gradient-to-r from-cyan-100/90 to-blue-50/70'
                            : 'text-gray-700 bg-white hover:text-gray-900 hover:shadow-md'
                        }`}
                      >
                        <span className={`${sysMenuOpen ? 'text-cyan-600' : 'text-gray-500'} transition-colors`}>{entry.icon}</span>
                        <span className="flex-1 text-left">{entry.group}</span>
                        <ChevronDown
                          size={14}
                          className={`text-gray-400 transition-transform duration-300 ${
                            sysMenuOpen ? 'rotate-180' : ''
                          }`}
                        />
                      </button>
                      {sysMenuOpen && (
                        <div style={{ marginLeft: '28px', paddingLeft: '16px', borderLeft: '2px solid #e5e7eb', marginTop: '8px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                          {visibleChildren.map((child) => {
                            const active = isActive(child);
                            return (
                              <Link
                                key={child.path}
                                to={child.path}
                                style={{ padding: '12px 12px' }}
                                className={`group flex items-center gap-3 rounded-2xl text-[13px] font-medium transition-all duration-300 ${
                                  active
                                    ? `${child.gradient} text-gray-900 shadow-sm ring-1 ring-white/60`
                                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50/80'
                                }`}
                              >
                                <span className={`${active ? (child.iconActive || 'text-gray-800') : 'text-gray-500 group-hover:scale-110'} transition-all duration-300`}>
                                  {child.icon}
                                </span>
                                <span>{child.label}</span>
                              </Link>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                }
                const item = entry as MenuItem;
                if (item.adminOnly && !isAdmin) return null;
                const active = isActive(item);
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    style={{
                      padding: '14px 16px',
                      borderRadius: '16px',
                      boxShadow: active
                        ? '0 2px 12px rgba(0,0,0,0.06), 0 0 0 1px rgba(0,0,0,0.04)'
                        : '0 1px 4px rgba(0,0,0,0.04), 0 0 0 1px rgba(0,0,0,0.03)',
                    }}
                    className={`group flex items-center gap-3 text-[14px] font-medium transition-all duration-300 relative overflow-hidden ${
                      active
                        ? `${item.gradient} text-gray-900`
                        : 'text-gray-700 bg-white hover:text-gray-900 hover:shadow-md'
                    }`}
                  >
                    <span className={`${active ? (item.iconActive || 'text-gray-800') : 'text-gray-500 group-hover:scale-110'} transition-all duration-300`}>
                      {item.icon}
                    </span>
                    <span className="relative z-10">{item.label}</span>
                    {item.path === '/app' && (
                      <div className="ml-auto flex gap-1.5 opacity-60">
                        <span className="w-5 h-5 rounded-md bg-white border border-gray-200 shadow-sm flex items-center justify-center text-[10px] text-gray-500">⌘</span>
                        <span className="w-5 h-5 rounded-md bg-white border border-gray-200 shadow-sm flex items-center justify-center text-[10px] text-gray-500">M</span>
                      </div>
                    )}
                  </Link>
                );
              })}
            </div>
          </div>
        </nav>

        {/* Bottom Section */}
        <div style={{ padding: '8px 20px 40px 20px', marginTop: '-24px' }} className="bg-gradient-to-t from-white via-white to-transparent">
          {/* User section */}
          {currentUser && (
            <div className="relative border-t border-gray-100/80 pt-3 mt-1">
              <button
                onClick={() => setShowUserMenu(!showUserMenu)}
                className="w-full flex items-center gap-3 px-2 py-2.5 rounded-2xl hover:bg-gray-50/80 transition-all duration-300 group"
              >
                {currentUser.avatar_url ? (
                  <img
                    src={currentUser.avatar_url}
                    alt="avatar"
                    className="w-10 h-10 rounded-[14px] object-cover ring-1 ring-black/5 shadow-sm transition-transform duration-300 group-hover:scale-105"
                  />
                ) : (
                  <div className="w-10 h-10 rounded-[14px] bg-gradient-to-br from-gray-800 to-gray-900 flex items-center justify-center text-white text-sm font-bold ring-1 ring-black/5 shadow-sm shadow-gray-900/20 transition-transform duration-300 group-hover:scale-105">
                    {currentUser.username.charAt(0).toUpperCase()}
                  </div>
                )}
                <div className="flex-1 text-left min-w-0">
                  <div className="text-[14px] font-semibold text-gray-800 leading-tight truncate">
                    {currentUser.display_name || currentUser.username}
                  </div>
                  <div className="text-[12px] font-medium text-gray-400 mt-0.5">
                    {currentUser.role === 'admin' ? 'Administrator' : 'User'}
                  </div>
                </div>
                <ChevronDown
                  size={16}
                  className={`text-gray-400 transition-transform duration-300 flex-shrink-0 ${
                    showUserMenu ? 'rotate-180' : ''
                  }`}
                />
              </button>
              {showUserMenu && (
                <div className="absolute bottom-full left-0 right-0 mb-6 bg-white/95 backdrop-blur-xl rounded-2xl shadow-[0_8px_30px_rgb(0,0,0,0.12)] border border-gray-100/80 p-2 z-50">
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-[13px] font-medium text-red-600 hover:bg-red-50/80 hover:text-red-700 transition-all duration-200"
                  >
                    <LogOut size={16} />
                    <span>退出登录</span>
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </aside>

      {/* ── Main content ── */}
      <main className="flex-1 bg-white/95 backdrop-blur-2xl rounded-[1.5rem] overflow-hidden shadow-[0_8px_30px_rgb(0,0,0,0.04)] ring-1 ring-gray-100/50 flex flex-col">
        <div style={{ padding: '36px 36px' }} className="flex-1 overflow-auto h-full">
          <Outlet />
        </div>
      </main>
    </div>
  );
}

export default Layout;
