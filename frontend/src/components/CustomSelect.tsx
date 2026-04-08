import { useState, useRef, useEffect } from 'react';

interface Option {
  value: string;
  label: string;
}

interface CustomSelectProps {
  options: Option[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  error?: boolean;
}

export default function CustomSelect({ options, value, onChange, placeholder, error }: CustomSelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const selected = options.find(o => o.value === value);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  return (
    <div ref={ref} style={{ position: 'relative', width: '100%' }}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        style={{
          width: '100%',
          padding: '10px 36px 10px 14px',
          background: open ? '#ffffff' : '#f9fafb',
          border: `1.5px solid ${error ? '#f87171' : open ? '#2563eb' : '#d1d5db'}`,
          borderRadius: '8px',
          fontSize: '14px',
          color: selected ? '#111827' : '#9ca3af',
          textAlign: 'left',
          cursor: 'pointer',
          outline: 'none',
          transition: 'all 0.2s',
          boxShadow: open ? '0 0 0 3px rgba(37,99,235,0.1)' : 'none',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}
      >
        {selected ? selected.label : placeholder || '请选择...'}
      </button>
      {/* Arrow icon */}
      <svg
        style={{
          position: 'absolute',
          right: '12px',
          top: '50%',
          transform: `translateY(-50%) rotate(${open ? '180deg' : '0deg'})`,
          transition: 'transform 0.2s',
          pointerEvents: 'none',
        }}
        width="14" height="14" viewBox="0 0 14 14" fill="none"
      >
        <path d="M3.5 5.25L7 8.75L10.5 5.25" stroke="#6b7280" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>

      {open && (
        <div style={{
          position: 'absolute',
          top: 'calc(100% + 4px)',
          left: 0,
          right: 0,
          background: '#ffffff',
          border: '1px solid #e5e7eb',
          borderRadius: '8px',
          boxShadow: '0 8px 24px rgba(0,0,0,0.12)',
          zIndex: 1000,
          maxHeight: '240px',
          overflowY: 'auto',
          padding: '4px',
        }}>
          {options.map(opt => (
            <div
              key={opt.value}
              onClick={() => { onChange(opt.value); setOpen(false); }}
              style={{
                padding: '9px 12px',
                fontSize: '14px',
                color: opt.value === value ? '#2563eb' : '#374151',
                background: opt.value === value ? '#eff6ff' : 'transparent',
                borderRadius: '6px',
                cursor: 'pointer',
                fontWeight: opt.value === value ? 600 : 400,
                transition: 'background 0.15s',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
              onMouseEnter={e => {
                if (opt.value !== value) (e.currentTarget as HTMLDivElement).style.background = '#f3f4f6';
              }}
              onMouseLeave={e => {
                (e.currentTarget as HTMLDivElement).style.background = opt.value === value ? '#eff6ff' : 'transparent';
              }}
            >
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{opt.label}</span>
              {opt.value === value && (
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" style={{ flexShrink: 0, marginLeft: '8px' }}>
                  <path d="M2.5 7.5L5.5 10.5L11.5 3.5" stroke="#2563eb" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
