import { env } from '$env/dynamic/private';

const API_URL = env.API_URL ?? 'http://localhost:8080';

/**
 * apiFetch llama al backend Go reenviando la cookie de sesión del usuario.
 *
 * Las rutas del backend bajo `/api/surveys` exigen una sesión válida, así que
 * cada request server-side debe propagar el token. Si se envía un body sin
 * Content-Type, se asume JSON.
 */
export function apiFetch(
	path: string,
	token: string | undefined,
	init: RequestInit = {}
): Promise<Response> {
	const headers = new Headers(init.headers);
	if (token) headers.set('Cookie', `session=${token}`);
	if (init.body && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}
	return fetch(`${API_URL}${path}`, { ...init, headers });
}
