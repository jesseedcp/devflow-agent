interface StatusBadgeProps {
  label: string;
  tone?: string;
  title?: string;
}

export function StatusBadge({ label, tone = 'tone-info', title }: StatusBadgeProps) {
  return (
    <span className={`badge ${tone}`} title={title ?? label}>
      {label}
    </span>
  );
}
