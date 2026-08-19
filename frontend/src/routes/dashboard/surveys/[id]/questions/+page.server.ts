import { redirect, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies, params }) => {
  if (!locals.user) redirect(302, '/login');

  const session = cookies.get('session');

  // Traemos la encuesta para mostrar el título y verificar acceso
  const surveyRes = await fetch(`${API}/api/surveys/${params.id}`, {
    headers: { Cookie: `session=${session}` }
  });

  if (!surveyRes.ok) redirect(302, '/dashboard/surveys');
  const survey = await surveyRes.json();

  // Verificamos write access con PATCH vacío
  let canEdit = false;
  try {
    const checkRes = await fetch(`${API}/api/surveys/${params.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
      body: JSON.stringify({})
    });
    canEdit = checkRes.status !== 403;
  } catch { canEdit = false; }

  // Traemos las preguntas
  const questionsRes = await fetch(`${API}/api/surveys/${params.id}/questions`, {
    headers: { Cookie: `session=${session}` }
  });

  const questions = questionsRes.ok ? await questionsRes.json() : [];

  return { survey, questions, canEdit };
};

export const actions: Actions = {
  createQuestion: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const payload = form.get('payload')?.toString() ?? '';

    if (!payload) return fail(400, { createError: 'Datos requeridos' });

    let body: Record<string, unknown>;
    try { body = JSON.parse(payload); }
    catch { return fail(400, { createError: 'JSON inválido' }); }

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/surveys/${params.id}/questions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
      body: JSON.stringify(body)
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      return fail(res.status, { createError: resBody.error ?? 'No se pudo crear la pregunta' });
    }

    return { createSuccess: true };
  },

  updateQuestion: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const qid     = form.get('qid')?.toString() ?? '';
    const payload = form.get('payload')?.toString() ?? '';

    if (!qid || !payload) return fail(400, { updateError: 'Datos requeridos' });

    let body: Record<string, unknown>;
    try { body = JSON.parse(payload); }
    catch { return fail(400, { updateError: 'JSON inválido' }); }

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/surveys/${params.id}/questions/${qid}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
      body: JSON.stringify(body)
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      return fail(res.status, { updateError: resBody.error ?? 'No se pudo actualizar la pregunta' });
    }

    return { updateSuccess: true };
  },

  deleteQuestion: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const qid = form.get('qid')?.toString() ?? '';

    if (!qid) return fail(400, { deleteError: 'ID requerido' });

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/surveys/${params.id}/questions/${qid}`, {
      method: 'DELETE',
      headers: { Cookie: `session=${session}` }
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      return fail(res.status, { deleteError: resBody.error ?? 'No se pudo eliminar la pregunta' });
    }

    return { deleteSuccess: true };
  },

  reorderQuestions: async ({ request, cookies, params }) => {
    const form = await request.formData();
    const order = form.get('order')?.toString() ?? '';

    if (!order) return fail(400, { reorderError: 'Orden requerido' });

    let orderedIDs: string[];
    try { orderedIDs = JSON.parse(order); }
    catch { return fail(400, { reorderError: 'JSON inválido' }); }

    const session = cookies.get('session');

    const res = await fetch(`${API}/api/surveys/${params.id}/questions/order`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
      body: JSON.stringify({ order: orderedIDs })
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      return fail(res.status, { reorderError: resBody.error ?? 'No se pudo reordenar' });
    }

    return { reorderSuccess: true };
  }
};