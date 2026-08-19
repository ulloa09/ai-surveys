import { error, fail } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey } from '$lib/types';
import type { Actions, PageServerLoad } from './$types';

/** load trae la encuesta a configurar. 404/403 se propagan como página de error. */
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
	// Guarda el modo de conversación, el system prompt y la configuración de terminación.
	updateConfig: async ({ request, params, cookies }) => {
		const token = cookies.get('session');
		const form = await request.formData();

		const mode = String(form.get('mode') ?? 'conversational');
		const systemPrompt = String(form.get('system_prompt') ?? '').trim();
		const terminationMode = String(form.get('termination_mode') ?? 'turn_limit');
		const turnLimitRaw = String(form.get('turn_limit') ?? '').trim();
		const timeEstimateRaw = String(form.get('time_estimate_minutes') ?? '').trim();
		const availableLanguages = form.getAll('available_languages').map((value) => String(value));
		const defaultLanguage = String(form.get('default_language') ?? 'es');

		if (mode !== 'form' && !systemPrompt) {
			return fail(400, { error: 'El system prompt es obligatorio para los modos B y C.' });
		}
		if (availableLanguages.length === 0) {
			return fail(400, { error: 'Selecciona al menos un idioma.' });
		}

		const payload: Record<string, unknown> = {
			mode,
			system_prompt: systemPrompt || null,
			termination_mode: terminationMode,
			available_languages: availableLanguages,
			default_language: defaultLanguage
		};
		if (terminationMode === 'turn_limit' || terminationMode === 'combination') {
			payload.turn_limit = turnLimitRaw ? Number(turnLimitRaw) : 12;
		}
		if (terminationMode === 'time_estimate' || terminationMode === 'combination') {
			if (!timeEstimateRaw) {
				return fail(400, { error: 'Indica la duración estimada en minutos.' });
			}
			payload.time_estimate_minutes = Number(timeEstimateRaw);
		}

		const res = await apiFetch(`/api/surveys/${params.id}`, token, {
			method: 'PATCH',
			body: JSON.stringify(payload)
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { error: body.error ?? 'No se pudo guardar la configuración.' });
		}
		return { updated: true };
	}
};
