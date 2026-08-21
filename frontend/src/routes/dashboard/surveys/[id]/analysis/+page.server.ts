import { error, fail, redirect } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey, SurveyAnalysis } from '$lib/types';
import { getPermissions } from '$lib/permissions';
import type { PageServerLoad, Actions } from './$types';

// Vista de análisis (#16). Solo lectura: el profesor (viewer) llega aquí igual
// que el admin, pero ninguna acción de ciclo de vida vive en esta página.
//
// El backend responde 404 al análisis mientras la encuesta no esté en
// 'analysing', 'complete' o 'failed'. No lo tratamos como un fallo silencioso:
// si alguien llega por URL directa antes de tiempo ve un 404 explícito, igual
// que el link que se esconde en el detalle.
export const load: PageServerLoad = async ({ locals, cookies, params }) => {
	if (!locals.user) redirect(302, '/login');

	const perms = getPermissions(locals.user.role);
	if (!perms.canViewResults) redirect(302, '/dashboard');

	const token = cookies.get('session');

	const [surveyRes, analysisRes] = await Promise.all([
		apiFetch(`/api/surveys/${params.id}`, token),
		apiFetch(`/api/surveys/${params.id}/analysis`, token)
	]);

	if (surveyRes.status === 403 || analysisRes.status === 403) {
		error(403, 'No tienes acceso a esta encuesta.');
	}
	if (surveyRes.status === 404) error(404, 'Encuesta no encontrada.');
	if (!surveyRes.ok) error(surveyRes.status, 'No se pudo cargar la encuesta.');

	if (analysisRes.status === 404) {
		error(404, 'El análisis de esta encuesta todavía no está disponible. Ciérrala para generarlo.');
	}
	if (!analysisRes.ok) error(analysisRes.status, 'No se pudo cargar el análisis.');

	const survey: Survey = await surveyRes.json();
	const analysis: SurveyAnalysis = await analysisRes.json();

	return { survey, analysis };
};

// retry vuelve a encolar el Analysis Engine para una encuesta en 'failed' o
// 'complete' — ver AnalysisService.AnalyseSurvey.
export const actions: Actions = {
	retry: async ({ cookies, params }) => {
		const token = cookies.get('session');
		const res = await apiFetch(`/api/surveys/${params.id}/analysis/retry`, token, { method: 'POST' });
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { retryError: body.error ?? 'No se pudo reintentar el análisis' });
		}
		return { retrySuccess: true };
	}
};
