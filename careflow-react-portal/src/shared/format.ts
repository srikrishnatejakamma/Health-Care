export const label = (value: unknown) =>
  String(value ?? '')
    .replace(/[._-]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
export const shortId = (value: unknown) => {
  const s = String(value ?? '');
  return s.length > 14 ? `${s.slice(0, 8)}…${s.slice(-4)}` : s;
};
export const date = (
  value: unknown,
  options: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' },
) => {
  const d = new Date(String(value));
  return Number.isNaN(d.getTime()) ? '—' : new Intl.DateTimeFormat('en', options).format(d);
};
export const time = (value: unknown) => date(value, { hour: 'numeric', minute: '2-digit' });
export const initials = (name: string) =>
  name
    .split(' ')
    .slice(0, 2)
    .map((w) => w[0])
    .join('')
    .toUpperCase();
export const bytes = (value: number) =>
  value > 2 ** 60 ? 'Not configured' : `${(value / 1024 / 1024).toFixed(1)} MB`;
export const displayName = (name: string | undefined, masked: boolean) =>
  !name ? 'Patient unavailable' : masked ? `${initials(name)} · Protected` : name;
export const since = (value: string) => {
  const minutes = Math.floor((Date.now() - new Date(value).getTime()) / 60000);
  return minutes < 1
    ? 'Just now'
    : minutes < 60
      ? `${minutes}m ago`
      : minutes < 1440
        ? `${Math.floor(minutes / 60)}h ago`
        : `${Math.floor(minutes / 1440)}d ago`;
};
