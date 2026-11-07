// pattern: Imperative Shell

import type {LucideIcon} from 'lucide-react';
import styles from './StatusBadge.module.css';

export type StatusBadgeProps = Readonly<{
  label: string;
  tone: 'info' | 'neutral' | 'success' | 'warning' | 'danger';
  icon?: LucideIcon;
  compact?: boolean;
}>;

export function StatusBadge({label, tone, icon: Icon, compact = false}: StatusBadgeProps) {
  return (
    <span data-status-badge="true" className={`${styles.badge} ${styles[tone]} ${compact ? styles.compact : ''}`}>
      {Icon ? <Icon aria-hidden="true" size={compact ? 13 : 14} strokeWidth={2} /> : null}
      <span>{label}</span>
    </span>
  );
}
