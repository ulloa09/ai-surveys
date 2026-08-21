import { apiFetch } from '$lib/server/api';
import type { Survey, SurveyStats } from '$lib/types';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ cookies, url }) => {
	const token = cookies.get('session');
	const showArchived = url.searchParams.get('archived') === 'true';
	const query = showArchived ? '?include_archived=true' : '';

	// Los agregados de TODAS las encuestas visibles llegan en una sola llamada
	// (#16), en vez de un fetch por encuesta.
	const [res, statsRes] = await Promise.all([
		apiFetch(`/api/surveys${query}`, token),
		apiFetch(`/api/surveys/stats${query}`, token)
	]);

	if (!res.ok) return { surveys: [], showArchived };

	const surveys: Survey[] = await res.json();

	const stats: SurveyStats[] = statsRes.ok ? await statsRes.json() : [];
	const statsBySurvey = new Map(stats.map((s) => [s.survey_id, s]));
	for (const survey of surveys) {
		survey.stats = statsBySurvey.get(survey.id);
	}

	return { surveys, showArchived };
};

export const actions: Actions = {
	// Duplicar una encuesta desde la lista
	duplicate: async ({ request, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();
		const id = form.get('id')?.toString() ?? '';

		if (!id) return { error: 'ID requerido' };

		const res = await apiFetch(`/api/surveys/${id}/duplicate`, token, { method: 'POST' });

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return { error: body.error ?? 'No se pudo duplicar la encuesta' };
		}

		return { success: true };
	},

	// Borrar una encuesta desde la lista
	delete: async ({ request, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();
		const id = form.get('id')?.toString() ?? '';

		if (!id) return { error: 'ID requerido' };

		const res = await apiFetch(`/api/surveys/${id}`, token, { method: 'DELETE' });

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return { error: body.error ?? 'No se pudo eliminar la encuesta' };
		}

		return { success: true };
	}
};
