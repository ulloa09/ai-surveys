<script lang="ts">
	import { page } from '$app/stores';
	import type { LayoutData } from './$types';
	import { ROLE_LABELS } from '$lib/types';
	import InstitutionalHeader from '$lib/components/shared/InstitutionalHeader.svelte';
	import InstitutionalFooter from '$lib/components/shared/InstitutionalFooter.svelte';

	let { data, children }: { data: LayoutData; children: import('svelte').Snippet } = $props();

	const navItems = [
		{ href: '/admin', label: 'Inicio' },
		{ href: '/admin/surveys', label: 'Encuestas' }
	];

	const headerUser = $derived({
		display_name: data.user.display_name,
		role_label: ROLE_LABELS[data.user.role as keyof typeof ROLE_LABELS] ?? data.user.role
	});

	// El endpoint /api/logout vale para toda la app; la acción ?/logout del
	// layout anterior solo existía en /admin, así que fallaba en /admin/surveys.
	async function logout() {
		await fetch('/api/logout', { method: 'POST' });
		window.location.href = '/login';
	}
</script>

<div class="shell">
	<InstitutionalHeader
		{navItems}
		currentPath={$page.url.pathname}
		user={headerUser}
		onLogout={logout}
		homeHref="/admin"
	/>

	<main>
		<div class="main-inner">
			{@render children()}
		</div>
	</main>

	<InstitutionalFooter />
</div>

<style>
	.shell {
		display: flex;
		flex-direction: column;
		min-height: 100vh;
	}

	main {
		flex: 1;
		background: var(--bg);
	}

	.main-inner {
		max-width: var(--container);
		margin: 0 auto;
		padding: 2rem;
	}
</style>
