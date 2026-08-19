import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals }) => {
	if (!locals.user) redirect(302, '/login');
	// Alumno: sin acceso a ningún panel de administración (Fase 3 — RBAC).
	// /dashboard ya sabe mostrar la pantalla de "sin acceso" para este rol.
	if (locals.user.role === 'alumno') redirect(302, '/dashboard');
	return { user: locals.user };
};
