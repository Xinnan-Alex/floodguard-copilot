const BACKEND_URL = import.meta.env.VITE_BACKEND_URL || 'http://localhost:3400';

export function backendUrl(path: string): string {
  return `${BACKEND_URL}${path}`;
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(backendUrl(path), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`Backend error: ${res.status}`);
  return res.json();
}
