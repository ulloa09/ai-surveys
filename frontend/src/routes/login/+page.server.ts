import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

function setSessionCookie(response: Response, cookies: any) {
	const setCookie = response.headers.get('set-cookie');

	if (!setCookie) return;

	const match = setCookie.match(/session=([^;]+)/);

	if (!match) return;

	cookies.set('session', match[1], {
		path: '/',
		httpOnly: true,
		sameSite: 'strict',
		maxAge: 60 * 60 * 24
	});
}

// safeRedirectTarget valida el parámetro ?redirect= antes de usarlo: debe ser
// una ruta relativa interna (empezar con "/" y no con "//" ni "/\", que los
// navegadores tratan como protocol-relative hacia un host externo). Cualquier
// otra cosa se descarta y cae al default — evita un open redirect si alguien
// arma un link de login con ?redirect=https://sitio-malicioso.com.
function safeRedirectTarget(raw: string | null, fallback: string): string {
	if (!raw) return fallback;
	if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return fallback;
	return raw;
}

export const load: PageServerLoad = async ({ cookies, fetch, url }) => {
	const session = cookies.get('session');
	// Se pasa tal cual al componente para meterlo como campo oculto del form:
	// action="?/login" resuelve como una URL RELATIVA a la query actual, así
	// que reemplaza — no conserva — el ?redirect= de la URL de esta página.
	// Sin el campo oculto, el valor se perdía y todo login terminaba en
	// /dashboard sin importar de dónde vino el usuario (bug reportado: un
	// alumno que venía de abrir una encuesta nunca volvía a ella).
	const redirectTo = url.searchParams.get('redirect');

	if (session) {
		try {
			const res = await fetch(`${API}/api/admin/me`, {
				headers: {
					Cookie: `session=${session}`
				}
			});

			if (res.ok) {
				redirect(302, safeRedirectTarget(redirectTo, '/dashboard'));
			}
		} catch {}
	}

	return { redirect: redirectTo };
};

export const actions: Actions = {
	login: async ({ request, fetch, cookies }) => {
		const form = await request.formData();

		const email = form.get('email')?.toString().trim() ?? '';
		const password = form.get('password')?.toString().trim() ?? '';
		// Viene de un campo oculto del form, NO de la URL: action="?/login" no
		// conserva el ?redirect= de la página (ver comentario en load()).
		const redirectTo = form.get('redirect')?.toString() ?? null;

		if (!email || !password) {
			return fail(400, {
				error: 'Completa todos los campos',
				email
			});
		}

		let res: Response;

		try {
			res = await fetch(`${API}/api/auth/login`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					email,
					password
				}),
				credentials: 'include'
			});
		} catch {
			return fail(503, {
				error: 'No se pudo conectar al servidor',
				email
			});
		}

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));

			return fail(res.status, {
				error: body.error ?? 'Credenciales incorrectas',
				email
			});
		}

		setSessionCookie(res, cookies);

		redirect(302, safeRedirectTarget(redirectTo, '/dashboard'));
	}
};
