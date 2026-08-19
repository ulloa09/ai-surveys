import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
  if (!locals.user) redirect(302, '/login');

  const session = cookies.get('session');
  const name  = url.searchParams.get('name')  ?? '';
  const order = url.searchParams.get('order') ?? 'desc';
  const year  = url.searchParams.get('year')  ?? '';
  const month = url.searchParams.get('month') ?? '';

  const params = new URLSearchParams();
  if (name)  params.set('name', name);
  if (order !== 'desc') params.set('order', order);
  if (year)  params.set('year', year);
  if (month) params.set('month', month);

  const qs = params.toString();
  const res = await fetch(`${API}/api/admin/teams${qs ? '?' + qs : ''}`, {
    headers: { Cookie: `session=${session}` }
  });

  if (!res.ok) return { teams: [], name, order, year, month };

  const teams = await res.json();
  return { teams, name, order, year, month };
};

export const actions: Actions = {
  createTeam: async ({ request, cookies }) => {
    const form = await request.formData();
    const name = form.get('name')?.toString().trim() ?? '';

    if (!name) return fail(400, { error: 'El nombre es requerido' });

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/admin/teams`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session=${session}`
      },
      body: JSON.stringify({ name })
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return fail(res.status, { error: body.error ?? 'No se pudo crear el equipo' });
    }

    return { success: true };
  }
};