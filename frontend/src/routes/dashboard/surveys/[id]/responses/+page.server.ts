import { error, redirect } from '@sveltejs/kit';
import { apiFetch } from '$lib/server/api';
import type { Survey, SurveyResponses } from '$lib/types';
import { getPermissions } from '$lib/permissions';
import type { PageServerLoad } from './$types';

// Visor de respuestas individuales: a diferencia de la vista de análisis, no
// depende de que el Analysis Engine haya corrido — la data cruda existe
// desde que hay respuestas enviadas, en cualquier estado de la encuesta.
export const load: PageServerLoad = async ({ locals, cookies, params }) => {
	if (!locals.user) redirect(302, '/login');

	const perms = getPermissions(locals.user.role);
	if (!perms.canViewResults) redirect(302, '/dashboard');

	const token = cookies.get('session');

	const [surveyRes, responsesRes] = await Promise.all([
		apiFetch(`/api/surveys/${params.id}`, token),
		apiFetch(`/api/surveys/${params.id}/responses`, token)
	]);

	if (surveyRes.status === 403 || responsesRes.status === 403) {
		error(403, 'No tienes acceso a esta encuesta.');
	}
	if (surveyRes.status === 404) error(404, 'Encuesta no encontrada.');
	if (!surveyRes.ok) error(surveyRes.status, 'No se pudo cargar la encuesta.');
	if (!responsesRes.ok) error(responsesRes.status, 'No se pudieron cargar las respuestas.');

	const survey: Survey = await surveyRes.json();
	const responses: SurveyResponses = await responsesRes.json();

	return { survey, responses };
};
