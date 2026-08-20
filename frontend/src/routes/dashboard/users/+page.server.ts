import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	if (!locals.user) redirect(302, '/login');
	if (locals.user.role !== 'super_admin') redirect(302, '/dashboard');

	const session = cookies.get('session');
	const search = url.searchParams.get('search') ?? '';
	const role = url.searchParams.get('role') ?? '';

	const params = new URLSearchParams();
	if (search) params.set('search', search);
	if (role) params.set('role', role);

	const qs = params.toString();
	const res = await fetch(`${API}/api/admin/users${qs ? '?' + qs : ''}`, {
		headers: { Cookie: `session=${session}` }
	});

	if (!res.ok) return { users: [], search, role };

	const users = await res.json();
	return { users, search, role };
};

export const actions: Actions = {
	createUser: async ({ request, cookies }) => {
		const form = await request.formData();
		const email = form.get('email')?.toString().trim() ?? '';
		const password = form.get('password')?.toString().trim() ?? '';
		const display_name = form.get('display_name')?.toString().trim() ?? '';
		const role = form.get('role')?.toString() ?? '';

		if (!email || !password || !display_name || !role) {
			return fail(400, { error: 'Todos los campos son requeridos' });
		}

		const session = cookies.get('session');

		const res = await fetch(`${API}/api/admin/users`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Cookie: `session=${session}`
			},
			body: JSON.stringify({ email, password, display_name, role })
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { error: body.error ?? 'No se pudo crear el usuario' });
		}

		return { success: true };
	}
};
