import { redirect, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies }) => {
  if (!locals.user) redirect(302, '/login');
  if (locals.user.role !== 'super_admin') redirect(302, '/dashboard');

  const session = cookies.get('session');
  const res = await fetch(`${API}/api/admin/settings`, {
    headers: { Cookie: `session=${session}` }
  });
  const settings = res.ok ? await res.json() : {};
  return { settings };
};

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const form = await request.formData();
    const body: Record<string, string> = {};

    const activeProvider = form.get('active_provider')?.toString();
    if (activeProvider) body.active_provider = activeProvider;

    const claudeKey = form.get('claude_key')?.toString().trim();
    if (claudeKey) body.claude_key = claudeKey;

    const claudeModel = form.get('claude_model')?.toString();
    if (claudeModel) body.claude_model = claudeModel;

    const openaiKey = form.get('openai_key')?.toString().trim();
    if (openaiKey) body.openai_key = openaiKey;

    const openaiModel = form.get('openai_model')?.toString();
    if (openaiModel) body.openai_model = openaiModel;

    const session = cookies.get('session');
    const res = await fetch(`${API}/api/admin/settings`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session=${session}`
      },
      body: JSON.stringify(body)
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      return fail(res.status, { error: resBody.error ?? 'No se pudo guardar' });
    }

    return { success: true };
  }
};