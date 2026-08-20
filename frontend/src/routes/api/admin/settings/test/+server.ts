import type { RequestHandler } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const POST: RequestHandler = async ({ cookies }) => {
  const session = cookies.get('session');
  const res = await fetch(`${API}/api/admin/settings/test`, {
    method: 'POST',
    headers: { Cookie: `session=${session}` }
  });
  const body = await res.json();
  return new Response(JSON.stringify(body), {
    status: res.status,
    headers: { 'Content-Type': 'application/json' }
  });
};