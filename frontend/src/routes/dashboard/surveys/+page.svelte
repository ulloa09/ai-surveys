<script lang="ts">
	import { STATUS_LABELS } from '$lib/types';
	import EmptyState from '$lib/components/shared/EmptyState.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	function fmtDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}
</script>

<svelte:head>
	<title>Encuestas — AI Surveys</title>
</svelte:head>

<div class="header">
	<h1>Encuestas</h1>
	<a class="primary" href="/admin/surveys">Nueva encuesta</a>
</div>

{#if data.surveys.length === 0}
	<EmptyState
		title="No hay encuestas todavía"
		description="Crea tu primera encuesta para empezar a recopilar retroalimentación."
	/>
{:else}
	<table>
		<thead>
			<tr>
				<th>Título</th>
				<th>Estado</th>
				<th>Equipo</th>
				<th>Creada</th>
			</tr>
		</thead>
		<tbody>
			{#each data.surveys as survey (survey.id)}
				<tr>
					<td><a href="/dashboard/surveys/{survey.id}">{survey.title}</a></td>
					<td><span class="badge">{STATUS_LABELS[survey.status] ?? survey.status}</span></td>
					<td>{survey.team_name ?? '—'}</td>
					<td>{fmtDate(survey.created_at)}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}

<style>
	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 1.25rem;
	}

	h1 {
		font-size: 1.5rem;
		color: var(--blue-900);
		margin: 0;
	}

	.primary {
		background: var(--blue-900);
		border: 1px solid var(--blue-900);
		border-radius: var(--radius-pill);
		color: #fff;
		font-family: var(--font-display);
		font-size: 0.8rem;
		font-weight: 600;
		padding: 0.5rem 1.1rem;
		text-decoration: none;
	}

	.primary:hover {
		background: var(--blue-700);
		border-color: var(--blue-700);
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		overflow: hidden;
	}
	th,
	td {
		text-align: left;
		padding: 0.7rem 1rem;
		border-bottom: 1px solid var(--border);
	}
	th {
		font-family: var(--font-display);
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--blue-900);
		border-bottom: 2px solid var(--blue-400);
	}
	td a {
		color: var(--blue-600);
		text-decoration: none;
		font-weight: 500;
	}
	td a:hover {
		text-decoration: underline;
	}

	.badge {
		display: inline-block;
		padding: 0.15rem 0.6rem;
		background: var(--blue-50);
		border: 1px solid var(--border);
		border-radius: var(--radius-pill);
		font-family: var(--font-display);
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--muted);
	}
</style>
