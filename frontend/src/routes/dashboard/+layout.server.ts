import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

// Protege todas las rutas bajo /dashboard.
// Si no hay sesión válida redirige al login.
// El usuario ya viene validado desde hooks.server.ts — solo lo pasamos al layout.
export const load: LayoutServerLoad = async ({ locals }) => {
	if (!locals.user) {
		redirect(302, '/login');
	}

	return {
		user: locals.user
	};
};
