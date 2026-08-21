import { json } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { RequestHandler } from './$types';

// Proxy autenticado hacia GET /api/responses/{id} (issue #14): la vista de
// respuestas (#16) lo usa para cargar bajo demanda el transcript de un
// participante seleccionado, sin traer todos los turns en el listado inicial.
export const GET: RequestHandler = async ({ params, cookies }) => {
	const token = cookies.get('session');
	const res = await apiFetch(`/api/responses/${params.id}`, token);
	const body = await res.json().catch(() => ({}));
	return json(body, { status: res.status });
};
