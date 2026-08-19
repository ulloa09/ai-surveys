<script lang="ts">
	import type { PageData } from './$types';
	import { ROLE_LABELS } from '$lib/types';

	let { data }: { data: PageData } = $props();

	const roleLabel = $derived(
		ROLE_LABELS[data.user?.role as keyof typeof ROLE_LABELS] ?? data.user?.role
	);
</script>

<svelte:head>
	<title>Panel de administración — AI Surveys</title>
</svelte:head>

<div class="card">
	<h1>Panel de administración</h1>
	<p>
		Bienvenido, <strong>{data.user?.display_name}</strong>. Tu rol es
		<strong>{roleLabel}</strong>.
	</p>
</div>

<style>
	.card {
		background: var(--white);
		border: 1px solid var(--border);
		border-left: 3px solid var(--blue-400);
		border-radius: var(--radius-sm);
		padding: 1.75rem;
		box-shadow: var(--shadow-sm);
	}

	h1 {
		font-size: 1.5rem;
		color: var(--blue-900);
		margin: 0 0 0.5rem;
	}

	p {
		margin: 0;
		font-size: 0.9rem;
		color: var(--muted);
	}

	strong { color: var(--text); }
</style>
