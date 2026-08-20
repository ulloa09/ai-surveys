import { error, redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { PageServerLoad } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ params, url, cookies }) => {
	const responseId = url.searchParams.get('response') ?? '';

	if (!responseId) {
		error(400, 'Falta el ID de respuesta');
	}

	// Si esta encuesta exige sesión lo dice el backend, no esta página: las
	// anónimas ('full') se contestan sin cuenta. Se pide la encuesta con la
	// sesión si la hay, y el 401 de abajo cubre a quien navegue directo a esta
	// URL sin haber pasado por el landing.
	const session = cookies.get('session');
	const res = await fetch(`${API}/api/public/surveys/${params.token}`, {
		headers: session ? { Cookie: `session=${session}` } : {}
	});
	if (res.status === 401) {
		const returnTo = `/s/${params.token}`;
		redirect(302, `/login?redirect=${encodeURIComponent(returnTo)}`);
	}
	if (res.status === 403) {
		error(403, 'No perteneces al equipo al que se desplegó esta encuesta.');
	}
	if (res.status === 404) error(404, 'Encuesta no encontrada');
	if (!res.ok) error(res.status, 'No se pudo cargar la encuesta');

	const survey = await res.json();

	return { survey, responseId };
};
