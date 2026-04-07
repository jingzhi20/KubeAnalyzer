/**
 * Kubernetes / Cloud-Native style SVG icons
 * Inspired by the official Kubernetes icon set
 */

interface IconProps {
  size?: number;
  color?: string;
  className?: string;
}

// 中心: Kubernetes 舵轮 (Helm wheel)
export function K8sWheelIcon({ size = 40, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <g stroke={color} strokeWidth="2.5" fill="none">
        {/* 外圈 */}
        <circle cx="32" cy="32" r="24" />
        {/* 内圈 */}
        <circle cx="32" cy="32" r="8" />
        {/* 7 个辐条 */}
        {[0, 1, 2, 3, 4, 5, 6].map(i => {
          const angle = (i * 360) / 7 - 90;
          const rad = (angle * Math.PI) / 180;
          const x1 = 32 + 8 * Math.cos(rad);
          const y1 = 32 + 8 * Math.sin(rad);
          const x2 = 32 + 24 * Math.cos(rad);
          const y2 = 32 + 24 * Math.sin(rad);
          return <line key={i} x1={x1} y1={y1} x2={x2} y2={y2} />;
        })}
        {/* 7 个端点圆 */}
        {[0, 1, 2, 3, 4, 5, 6].map(i => {
          const angle = (i * 360) / 7 - 90;
          const rad = (angle * Math.PI) / 180;
          const cx = 32 + 24 * Math.cos(rad);
          const cy = 32 + 24 * Math.sin(rad);
          return <circle key={i} cx={cx} cy={cy} r="3.5" fill={color} />;
        })}
      </g>
    </svg>
  );
}

// Pod 图标
export function PodIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="8" y="12" width="48" height="40" rx="6" stroke={color} strokeWidth="3" />
      <rect x="18" y="24" width="10" height="16" rx="2" fill={color} opacity="0.7" />
      <rect x="34" y="24" width="10" height="16" rx="2" fill={color} opacity="0.7" />
      <line x1="32" y1="12" x2="32" y2="52" stroke={color} strokeWidth="1.5" strokeDasharray="3 2" />
    </svg>
  );
}

// 网络/Service 图标
export function ServiceIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <polygon points="32,6 58,20 58,44 32,58 6,44 6,20" stroke={color} strokeWidth="2.5" fill="none" />
      <circle cx="32" cy="32" r="8" stroke={color} strokeWidth="2" fill={color} fillOpacity="0.3" />
      <line x1="32" y1="24" x2="32" y2="6" stroke={color} strokeWidth="1.5" />
      <line x1="38" y1="36" x2="58" y2="44" stroke={color} strokeWidth="1.5" />
      <line x1="26" y1="36" x2="6" y2="44" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}

// 存储/PV 图标
export function StorageIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <ellipse cx="32" cy="16" rx="22" ry="8" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M10 16v16c0 4.4 9.8 8 22 8s22-3.6 22-8V16" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M10 32v16c0 4.4 9.8 8 22 8s22-3.6 22-8V32" stroke={color} strokeWidth="2.5" fill="none" />
      <ellipse cx="32" cy="32" rx="22" ry="8" stroke={color} strokeWidth="1" strokeDasharray="3 2" fill="none" />
    </svg>
  );
}

// 安全/Shield 图标
export function ShieldIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <path d="M32 4L8 16v16c0 14 10.7 27 24 32 13.3-5 24-18 24-32V16L32 4z" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M24 32l6 6 12-14" stroke={color} strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// 节点/Node 图标
export function NodeIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="6" y="10" width="52" height="36" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="6" y1="22" x2="58" y2="22" stroke={color} strokeWidth="1.5" />
      <circle cx="14" cy="16" r="2.5" fill={color} />
      <circle cx="22" cy="16" r="2.5" fill={color} />
      <circle cx="30" cy="16" r="2.5" fill={color} />
      <rect x="12" y="28" width="16" height="4" rx="1" fill={color} opacity="0.5" />
      <rect x="12" y="36" width="24" height="4" rx="1" fill={color} opacity="0.5" />
      <rect x="22" y="50" width="20" height="6" rx="2" stroke={color} strokeWidth="1.5" fill="none" />
      <line x1="32" y1="46" x2="32" y2="50" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}

// Deployment/工作负载 图标
export function DeploymentIcon({ size = 24, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="6" y="6" width="24" height="24" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <rect x="34" y="6" width="24" height="24" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <rect x="6" y="34" width="24" height="24" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <rect x="34" y="34" width="24" height="24" rx="4" stroke={color} strokeWidth="2.5" fill={color} fillOpacity="0.2" />
      <path d="M42 42l6 6 0-12" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// ---- 子案例图标 ----

// CrashLoop 图标 (循环箭头 + 感叹号)
export function CrashLoopIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <path d="M44 16A18 18 0 1 0 50 32" stroke={color} strokeWidth="3" strokeLinecap="round" fill="none" />
      <polygon points="50,24 50,40 56,32" fill={color} />
      <line x1="32" y1="24" x2="32" y2="36" stroke={color} strokeWidth="3" strokeLinecap="round" />
      <circle cx="32" cy="42" r="2" fill={color} />
    </svg>
  );
}

// HPA 图标 (水平缩放箭头)
export function HPAIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="22" y="14" width="20" height="36" rx="3" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M10 32H22M42 32H54" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <polygon points="8,32 14,27 14,37" fill={color} />
      <polygon points="56,32 50,27 50,37" fill={color} />
      <line x1="28" y1="24" x2="36" y2="24" stroke={color} strokeWidth="2" strokeLinecap="round" />
      <line x1="28" y1="32" x2="36" y2="32" stroke={color} strokeWidth="2" strokeLinecap="round" />
      <line x1="28" y1="40" x2="36" y2="40" stroke={color} strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

// Service 不通 (断开连接)
export function ServiceDownIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <circle cx="16" cy="32" r="10" stroke={color} strokeWidth="2.5" fill="none" />
      <circle cx="48" cy="32" r="10" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="26" y1="28" x2="38" y2="28" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="26" y1="36" x2="38" y2="36" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="30" y1="24" x2="34" y2="40" stroke={color} strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

// Ingress 图标
export function IngressIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <polygon points="32,8 56,24 56,48 32,56 8,48 8,24" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M8 24L32 32L56 24" stroke={color} strokeWidth="1.5" />
      <line x1="32" y1="32" x2="32" y2="56" stroke={color} strokeWidth="1.5" />
      <polygon points="32,14 28,20 36,20" fill={color} />
    </svg>
  );
}

// PVC Pending 图标
export function PVCIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <ellipse cx="32" cy="20" rx="18" ry="7" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M14 20v24c0 3.9 8 7 18 7s18-3.1 18-7V20" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="32" y1="32" x2="32" y2="40" stroke={color} strokeWidth="3" strokeLinecap="round" />
      <circle cx="32" cy="45" r="2" fill={color} />
    </svg>
  );
}

// Volume 挂载图标
export function VolumeIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="10" y="8" width="44" height="20" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <rect x="10" y="36" width="44" height="20" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="32" y1="28" x2="32" y2="36" stroke={color} strokeWidth="2" />
      <circle cx="46" cy="18" r="3" fill={color} />
      <circle cx="46" cy="46" r="3" fill={color} />
    </svg>
  );
}

// RBAC 图标 (钥匙+盾)
export function RBACIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <circle cx="24" cy="24" r="10" stroke={color} strokeWidth="2.5" fill="none" />
      <circle cx="24" cy="24" r="4" fill={color} fillOpacity="0.4" />
      <line x1="32" y1="28" x2="52" y2="48" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="44" y1="40" x2="50" y2="34" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="48" y1="44" x2="54" y2="38" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

// Secret 图标 (锁)
export function SecretIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="14" y="28" width="36" height="28" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <path d="M20 28V20a12 12 0 0 1 24 0v8" stroke={color} strokeWidth="2.5" fill="none" />
      <circle cx="32" cy="42" r="4" fill={color} />
      <line x1="32" y1="46" x2="32" y2="50" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

// Node NotReady 图标
export function NodeNotReadyIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="8" y="12" width="48" height="32" rx="4" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="24" y1="22" x2="40" y2="34" stroke={color} strokeWidth="3" strokeLinecap="round" />
      <line x1="40" y1="22" x2="24" y2="34" stroke={color} strokeWidth="3" strokeLinecap="round" />
      <rect x="24" y="48" width="16" height="5" rx="2" stroke={color} strokeWidth="1.5" fill="none" />
      <line x1="32" y1="44" x2="32" y2="48" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}

// 驱逐风暴图标
export function EvictionIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <rect x="8" y="16" width="20" height="14" rx="3" stroke={color} strokeWidth="2" fill="none" />
      <rect x="8" y="36" width="20" height="14" rx="3" stroke={color} strokeWidth="2" fill="none" />
      <path d="M34 23H52M34 43H52" stroke={color} strokeWidth="2" strokeLinecap="round" />
      <polygon points="52,18 58,23 52,28" fill={color} />
      <polygon points="52,38 58,43 52,48" fill={color} />
    </svg>
  );
}

// Job 超时图标 (时钟)
export function JobTimeoutIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <circle cx="32" cy="34" r="22" stroke={color} strokeWidth="2.5" fill="none" />
      <line x1="32" y1="34" x2="32" y2="20" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="32" y1="34" x2="42" y2="40" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="26" y1="8" x2="38" y2="8" stroke={color} strokeWidth="2.5" strokeLinecap="round" />
      <line x1="32" y1="8" x2="32" y2="12" stroke={color} strokeWidth="2" />
    </svg>
  );
}

// 滚动更新图标 (循环箭头)
export function RollingUpdateIcon({ size = 22, color = '#fff' }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <path d="M48 20A20 20 0 0 1 48 44" stroke={color} strokeWidth="2.5" strokeLinecap="round" fill="none" />
      <path d="M16 44A20 20 0 0 1 16 20" stroke={color} strokeWidth="2.5" strokeLinecap="round" fill="none" />
      <polygon points="48,44 42,38 54,38" fill={color} />
      <polygon points="16,20 22,26 10,26" fill={color} />
      <rect x="24" y="24" width="16" height="16" rx="3" stroke={color} strokeWidth="2" fill={color} fillOpacity="0.2" />
    </svg>
  );
}
