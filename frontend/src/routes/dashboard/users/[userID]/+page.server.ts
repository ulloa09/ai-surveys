import { redirect, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { env } from '$env/dynamic/private';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals, cookies, params }) => {
	if (!locals.user) redirect(302, '/login');
	if (locals.user.role !== 'super_admin') redirect(302, '/dashboard');

	const session = cookies.get('session');

	const res = await fetch(`${API}/api/admin/users/${params.userID}`, {
		headers: { Cookie: `session=${session}` }
	});

	if (!res.ok) redirect(302, '/dashboard/users');

	const targetUser = await res.json();
	return { targetUser };
};

export const actions: Actions = {
	updateUser: async ({ request, cookies, params }) => {
		const form = await request.formData();
		const email = form.get('email')?.toString().trim() ?? '';
		const display_name = form.get('display_name')?.toString().trim() ?? '';

		if (!email || !display_name) {
			return fail(400, { updateError: 'Correo y nombre son requeridos' });
		}

		const session = cookies.get('session');

		const res = await fetch(`${API}/api/admin/users/${params.userID}`, {
			method: 'PATCH',
			headers: {
				'Content-Type': 'application/json',
				Cookie: `session=${session}`
			},
			body: JSON.stringify({ email, display_name })
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { updateError: body.error ?? 'No se pudo actualizar el usuario' });
		}

		return { updateSuccess: true };
	},

	changeRole: async ({ request, cookies, params }) => {
		const form = await request.formData();
		const role = form.get('role')?.toString() ?? '';

		if (!role) return fail(400, { roleError: 'El rol es requerido' });

		const session = cookies.get('session');

		const res = await fetch(`${API}/api/admin/users/${params.userID}/role`, {
			method: 'PATCH',
			headers: {
				'Content-Type': 'application/json',
				Cookie: `session=${session}`
			},
			body: JSON.stringify({ role })
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { roleError: body.error ?? 'No se pudo cambiar el rol' });
		}

		return { roleSuccess: true };
	},

	resetPassword: async ({ request, cookies, params }) => {
		const form = await request.formData();
		const new_password = form.get('new_password')?.toString() ?? '';

		if (!new_password || new_password.length < 8) {
			return fail(400, { passwordError: 'La contraseña debe tener al menos 8 caracteres' });
		}

		const session = cookies.get('session');

		const res = await fetch(`${API}/api/admin/users/${params.userID}/password`, {
			method: 'PATCH',
			headers: {
				'Content-Type': 'application/json',
				Cookie: `session=${session}`
			},
			body: JSON.stringify({ new_password })
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { passwordError: body.error ?? 'No se pudo restablecer la contraseña' });
		}

		return { passwordSuccess: true };
	},

	deleteUser: async ({ cookies, params }) => {
		const session = cookies.get('session');

		const res = await fetch(`${API}/api/admin/users/${params.userID}`, {
			method: 'DELETE',
			headers: { Cookie: `session=${session}` }
		});

		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			return fail(res.status, { deleteError: body.error ?? 'No se pudo eliminar el usuario' });
		}

		redirect(303, '/dashboard/users');
	}
};
