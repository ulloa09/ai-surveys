import type { PageServerLoad } from './$types';

// El usuario ya viene del layout server, no necesitamos hacer nada extra aquí.
// SvelteKit fusiona el data del layout con el de la page automáticamente.
export const load: PageServerLoad = async () => {
	return {};
};
