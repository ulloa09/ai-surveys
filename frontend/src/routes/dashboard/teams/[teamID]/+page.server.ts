import { redirect, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies, params, url }) => {
  if (!locals.user) redirect(302, '/login');

  const session = cookies.get('session');
  const member = url.searchParams.get('member') ?? '';

  const endpoint = member
    ? `${API}/api/admin/teams/${params.teamID}?member=${encodeURIComponent(member)}`
    : `${API}/api/admin/teams/${params.teamID}`;

  const res = await fetch(endpoint, {
    headers: { Cookie: `session=${session}` }
  });

  if (!res.ok) redirect(302, '/dashboard/teams');

  const team = await res.json();

  // Lista de otros equipos, para el selector de "mover miembro a…".
  // Si el usuario no tiene permiso de listar todos los equipos (no es
  // admin/super_admin), el backend igual devuelve solo los suyos.
  let otherTeams: { id: string; name: string }[] = [];
  const teamsRes = await fetch(`${API}/api/admin/teams`, {
    headers: { Cookie: `session=${session}` }
  });
  if (teamsRes.ok) {
    const allTeams = await teamsRes.json();
    otherTeams = (allTeams as { id: string; name: string }[]).filter((t) => t.id !== params.teamID);
  }

  return { team, member, otherTeams };
};

export const actions: Actions = {
  inviteMember: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString().trim() ?? '';

    if (!email) return fail(400, { inviteError: 'El correo es requerido' });

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/admin/teams/${params.teamID}/members`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session=${session}`
      },
      body: JSON.stringify({ email })
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return fail(res.status, { inviteError: body.error ?? 'No se pudo invitar al miembro' });
    }

    return { inviteSuccess: true };
  },

  removeMember: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const userID = form.get('user_id')?.toString() ?? '';

    if (!userID) return fail(400, { removeError: 'ID de usuario requerido' });

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/admin/teams/${params.teamID}/members/${userID}`, {
      method: 'DELETE',
      headers: { Cookie: `session=${session}` }
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return fail(res.status, { removeError: body.error ?? 'No se pudo remover al miembro' });
    }

    return { removeSuccess: true };
  },

  moveMember: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const userID = form.get('user_id')?.toString() ?? '';
    const toTeamID = form.get('to_team_id')?.toString() ?? '';

    if (!userID || !toTeamID) {
      return fail(400, { moveError: 'Elige un equipo destino' });
    }

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/admin/teams/${params.teamID}/members/${userID}/move`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Cookie: `session=${session}`
      },
      body: JSON.stringify({ to_team_id: toTeamID })
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return fail(res.status, { moveError: body.error ?? 'No se pudo mover al miembro' });
    }

    return { moveSuccess: true };
  }
};
