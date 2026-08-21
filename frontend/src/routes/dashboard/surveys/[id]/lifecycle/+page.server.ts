import { error, fail } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey, SurveyStats } from '$lib/types';
import type { Actions, PageServerLoad } from './$types';

/** load trae la encuesta cuyo ciclo de vida se administra en esta página. */
export const load: PageServerLoad = async ({ params, cookies }) => {
	const token = cookies.get('session');
	const res = await apiFetch(`/api/surveys/${params.id}`, token);

	if (res.status === 404) error(404, 'Encuesta no encontrada');
	if (res.status === 403) error(403, 'No tienes acceso a esta encuesta');
	if (!res.ok) error(res.status, 'No se pudo cargar la encuesta');

	const survey: Survey = await res.json();

	// Respuestas, tasa de completado y duración promedio (#16). Disponibles en
	// cualquier estado, no solo tras el análisis. Si el endpoint falla, la
	// tarjeta de resultados no se pinta — no vale tumbar la página de detalle.
	const statsRes = await apiFetch(`/api/surveys/${params.id}/stats`, token);
	const stats: SurveyStats | null = statsRes.ok ? await statsRes.json() : null;

	return { survey, stats };
};

export const actions: Actions = {
	// Guarda la programación (apertura/cierre automáticos) y el tope de respuestas.
	updateSchedule: async ({ request, params, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();

		const opensAtRaw = String(form.get('opens_at') ?? '').trim();
		const closesAtRaw = String(form.get('closes_at') ?? '').trim();
		const responseCapRaw = String(form.get('response_cap') ?? '').trim();

		const payload: Record<string, unknown> = {
			opens_at: opensAtRaw ? new Date(opensAtRaw).toISOString() : null,
			closes_at: closesAtRaw ? new Date(closesAtRaw).toISOString() : null,
			response_cap: responseCapRaw ? Number(responseCapRaw) : null
		};

		const res = await apiFetch(`/api/surveys/${params.id}`, token, {
			method: 'PATCH',
			body: JSON.stringify(payload)
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { scheduleError: body.error ?? 'No se pudo guardar la programación.' });
		}
		return { scheduleUpdated: true };
	},

	activate: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/activate`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { lifecycleError: body.error ?? 'No se pudo activar la encuesta.' });
		}
		return { lifecycleUpdated: true };
	},

	close: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/close`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { lifecycleError: body.error ?? 'No se pudo cerrar la encuesta.' });
		}
		return { lifecycleUpdated: true };
	},

	reopen: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/reopen`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { lifecycleError: body.error ?? 'No se pudo reabrir la encuesta.' });
		}
		return { lifecycleUpdated: true };
	},

	archive: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/archive`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { lifecycleError: body.error ?? 'No se pudo archivar la encuesta.' });
		}
		return { lifecycleUpdated: true };
	},

	// Re-encola el Analysis Engine (#15) para una encuesta en 'failed' (el job
	// no pudo completarse) o 'complete' (re-analizar sobrescribe resultados).
	retry: async ({ params, cookies }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/analysis/retry`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { lifecycleError: body.error ?? 'No se pudo reintentar el análisis.' });
		}
		return { lifecycleUpdated: true };
	}
};
