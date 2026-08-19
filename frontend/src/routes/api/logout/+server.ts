import { redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const POST: RequestHandler = async ({ cookies }) => {
  const session = cookies.get('session');

  if (session) {
    await fetch(`${API}/api/auth/logout`, {
      method: 'POST',
      headers: { Cookie: `session=${session}` }
    }).catch(() => {});
  }

  cookies.delete('session', { path: '/' });
  redirect(302, '/login');
};