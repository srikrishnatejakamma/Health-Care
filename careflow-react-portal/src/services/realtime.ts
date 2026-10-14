import { useEffect, useState } from 'react';
import { queryClient } from './apiClient';
import { useSession } from '../stores/session';

export function useRealtime() {
  const epoch = useSession((s) => s.epoch);
  const mode = useSession((s) => s.mode);
  const [status, setStatus] = useState('connecting');
  useEffect(() => {
    if (mode !== 'live') {
      setStatus(mode === 'demo' ? 'demo' : 'offline');
      return;
    }
    const controller = new AbortController();
    let attempts = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;
    const run = async () => {
      const current = useSession.getState();
      try {
        setStatus(attempts ? 'reconnecting' : 'connecting');
        const res = await fetch('/api/v1/stream', {
          headers: { Authorization: `Bearer ${current.token}`, 'X-Tenant-ID': current.tenantId },
          signal: controller.signal,
          cache: 'no-store',
        });
        if (!res.ok || !res.body) throw new Error('Stream unavailable');
        setStatus('live');
        attempts = 0;
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          if (buffer.length > 8192) throw new Error('Stream frame exceeded limit');
          let index: number;
          while ((index = buffer.indexOf('\n\n')) >= 0) {
            const frame = buffer.slice(0, index);
            buffer = buffer.slice(index + 2);
            const line = frame.split('\n').find((x) => x.startsWith('data:'));
            if (line) {
              const event = JSON.parse(line.slice(5));
              if (event.tenant_id === useSession.getState().tenantId && useSession.getState().epoch === epoch)
                void queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] === epoch });
            }
          }
        }
        if (!disposed) throw new Error('Stream ended');
      } catch {
        if (disposed) return;
        setStatus('reconnecting');
        timer = setTimeout(run, Math.min(30_000, 1000 * 2 ** Math.min(attempts++, 5)));
      }
    };
    void run();
    return () => {
      disposed = true;
      controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [epoch, mode]);
  return status;
}
