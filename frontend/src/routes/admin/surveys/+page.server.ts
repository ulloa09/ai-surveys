import { fail } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey, Team } from '$lib/types';
import type { Actions, PageServerLoad } from './$types';

/**
 * load trae las encuestas visibles para el usuario y la lista de equipos
 * (necesaria para el selector del formulario de creación).
 */
export const load: PageServerLoad = async ({ cookies }) => {
	const token = cookies.get('session');

	const [surveysRes, teamsRes] = await Promise.all([
		apiFetch('/api/surveys', token),
		apiFetch('/api/admin/teams', token)
	]);

	const surveys: Survey[] = surveysRes.ok ? await surveysRes.json() : [];
	const teams: Team[] = teamsRes.ok ? await teamsRes.json() : [];

	return { surveys, teams };
};

export const actions: Actions = {
	// Crea una encuesta nueva en estado draft.
	create: async ({ request, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();

		const title = String(form.get('title') ?? '').trim();
		const teamId = String(form.get('team_id') ?? '');

		if (!title) return fail(400, { error: 'El título es obligatorio.' });
		if (!teamId) return fail(400, { error: 'Selecciona un equipo.' });

		const payload = {
			title,
			description: String(form.get('description') ?? '').trim() || null,
			team_id: teamId,
			anonymity_level: String(form.get('anonymity_level') ?? 'none'),
			allow_revisit: form.get('allow_revisit') === 'on',
			optional_registration: form.get('optional_registration') === 'on',
			// Este formulario rápido no ofrece selector de modo (#06) — crea en
			// Mode A (form, preguntas fijas), el único que no exige system_prompt.
			// El modo se puede cambiar después desde el panel de configuración
			// de la encuesta.
			mode: 'form'
		};

		const res = await apiFetch('/api/surveys', token, {
			method: 'POST',
			body: JSON.stringify(payload)
		});

		if (!res.ok) {
			return fail(res.status, { error: 'No se pudo crear la encuesta.' });
		}
		return { created: true };
	},

	// Duplica una encuesta existente (copia draft, sin respuestas).
	duplicate: async ({ request, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();
		const id = String(form.get('id') ?? '');

		const res = await apiFetch(`/api/surveys/${id}/duplicate`, token, { method: 'POST' });
		if (!res.ok) {
			return fail(res.status, { error: 'No se pudo duplicar la encuesta.' });
		}
		return { duplicated: true };
	},

	// Elimina una encuesta. El backend devuelve 409 si ya tiene respuestas.
	delete: async ({ request, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();
		const id = String(form.get('id') ?? '');

		const res = await apiFetch(`/api/surveys/${id}`, token, { method: 'DELETE' });
		if (res.status === 409) {
			return fail(409, { error: 'No se puede eliminar: la encuesta ya tiene respuestas.' });
		}
		if (!res.ok) {
			return fail(res.status, { error: 'No se pudo eliminar la encuesta.' });
		}
		return { deleted: true };
	}
};
