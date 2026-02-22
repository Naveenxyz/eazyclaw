import { useState } from 'react';
import { MessageSquare, Brain, Puzzle, Activity, Settings } from 'lucide-react';

interface IconRailProps {
  activeTab: string;
  onTabChange: (tab: string) => void;
  isConnected: boolean;
}

const NAV_ITEMS = [
  { id: 'Chat', icon: MessageSquare, label: 'Chat' },
  { id: 'Memory', icon: Brain, label: 'Memory' },
  { id: 'Skills', icon: Puzzle, label: 'Skills' },
  { id: 'Status', icon: Activity, label: 'Status' },
  { id: 'Settings', icon: Settings, label: 'Settings' },
];

export default function IconRail({ activeTab, onTabChange, isConnected }: IconRailProps) {
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  return (
    <nav
      className="flex flex-col items-center justify-between py-4"
      style={{
        width: 56,
        height: '100%',
        background: '#0f1117',
        borderRight: '1px solid rgba(139, 92, 246, 0.1)',
      }}
    >
      <div className="flex flex-col items-center gap-1">
        {NAV_ITEMS.map(({ id, icon: Icon, label }) => {
          const isActive = activeTab === id;
          return (
            <div key={id} className="relative">
              <button
                onClick={() => onTabChange(id)}
                onMouseEnter={() => setHoveredId(id)}
                onMouseLeave={() => setHoveredId(null)}
                className="relative flex items-center justify-center"
                style={{
                  width: 44,
                  height: 44,
                  borderRadius: 10,
                  borderLeft: isActive ? '3px solid #8b5cf6' : '3px solid transparent',
                  background: isActive ? 'rgba(139, 92, 246, 0.15)' : 'transparent',
                  color: isActive ? '#8b5cf6' : '#64748b',
                  transition: 'all 0.2s ease',
                }}
              >
                <Icon
                  size={20}
                  style={{
                    filter: isActive ? 'drop-shadow(0 0 6px rgba(139, 92, 246, 0.5))' : 'none',
                  }}
                />
              </button>
              {hoveredId === id && (
                <div
                  className="absolute top-1/2 -translate-y-1/2 whitespace-nowrap"
                  style={{
                    left: 52,
                    background: '#151720',
                    border: '1px solid rgba(139, 92, 246, 0.2)',
                    borderRadius: 6,
                    padding: '4px 10px',
                    fontSize: 12,
                    color: '#e2e8f0',
                    zIndex: 50,
                    pointerEvents: 'none',
                  }}
                >
                  {label}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Connection status dot */}
      <div className="flex flex-col items-center gap-1">
        <div
          className={`status-dot ${isConnected ? 'connected' : 'error'}`}
          title={isConnected ? 'Connected' : 'Disconnected'}
        />
      </div>
    </nav>
  );
}
