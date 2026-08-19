import { error, fail, redirect } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey } from '$lib/types';
import type { Actions, PageServerLoad } from './$types';

/** load trae la encuesta a editar. 404/403 se propagan como página de error. */
export const load: PageServerLoad = async ({ params, cookies }) => {
	const token = cookies.get('session');
	const res = await apiFetch(`/api/surveys/${params.id}`, token);

	if (res.status === 404) error(404, 'Encuesta no encontrada');
	if (res.status === 403) error(403, 'No tienes acceso a esta encuesta');
	if (!res.ok) error(res.status, 'No se pudo cargar la encuesta');

	const survey: Survey = await res.json();
	return { survey };
};

export const actions: Actions = {
	// Guarda los cambios del formulario de edición.
	update: async ({ request, params, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();

		const title = String(form.get('title') ?? '').trim();
		if (!title) return fail(400, { error: 'El título es obligatorio.' });

		const payload = {
			title,
			description: String(form.get('description') ?? '').trim() || null,
			anonymity_level: String(form.get('anonymity_level') ?? 'none'),
			allow_revisit: form.get('allow_revisit') === 'on',
			optional_registration: form.get('optional_registration') === 'on'
		};

		const res = await apiFetch(`/api/surveys/${params.id}`, token, {
			method: 'PATCH',
			body: JSON.stringify(payload)
		});

		if (res.status === 409) {
			return fail(409, {
				error: 'El nivel de anonimato no se puede cambiar porque la encuesta ya tiene respuestas.'
			});
		}
		if (!res.ok) {
			return fail(res.status, { error: 'No se pudieron guardar los cambios.' });
		}
		return { updated: true };
	},

	// Duplica esta encuesta y vuelve al listado.
	duplicate: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/duplicate`, token, { method: 'POST' });
		if (!res.ok) {
			return fail(res.status, { error: 'No se pudo duplicar la encuesta.' });
		}
		redirect(303, '/admin/surveys');
	},

	// Elimina esta encuesta. 409 si ya tiene respuestas.
	delete: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}`, token, { method: 'DELETE' });
		if (res.status === 409) {
			return fail(409, { error: 'No se puede eliminar: la encuesta ya tiene respuestas.' });
		}
		if (!res.ok) {
			return fail(res.status, { error: 'No se pudo eliminar la encuesta.' });
		}
		redirect(303, '/admin/surveys');
	}
};
