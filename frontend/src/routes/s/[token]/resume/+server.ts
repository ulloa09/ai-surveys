import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

// GET /s/<token>/resume?token=<resume_token>
//
// Proxy del lado del servidor hacia el backend Go (cuya URL es privada y no
// alcanzable desde el navegador). Valida el resume token guardado en
// localStorage y, si sigue vigente, devuelve el id de la respuesta en curso
// para que la landing muestre la opción "Continuar donde lo dejaste".
//
// Cualquier token inválido/expirado/desconocido se traduce a `{ found: false }`
// para que el cliente simplemente arranque una sesión nueva (criterio #09).
export const GET: RequestHandler = async ({ params, url }) => {
	const resumeToken = url.searchParams.get('token');
	if (!resumeToken) return json({ found: false });

	const res = await fetch(
		`${API}/api/public/surveys/${params.token}/resume?token=${encodeURIComponent(resumeToken)}`
	);
	if (!res.ok) return json({ found: false });

	const response = await res.json();
	return json({ found: true, responseId: response.id as string });
};
