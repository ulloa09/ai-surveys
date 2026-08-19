<script lang="ts">
	import { enhance } from '$app/forms';
	import { STATUS_LABELS, ANONYMITY_LABELS } from '$lib/types';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	let showCreate = $state(false);

	function fmtDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}
</script>

<svelte:head>
	<title>Encuestas — AI Surveys</title>
</svelte:head>

<div class="header">
	<h1>Encuestas</h1>
	<button class="primary" onclick={() => (showCreate = !showCreate)}>
		{showCreate ? 'Cancelar' : 'Nueva encuesta'}
	</button>
</div>

{#if form?.error}
	<p class="banner error" role="alert">{form.error}</p>
{:else if form?.created}
	<p class="banner ok">Encuesta creada.</p>
{:else if form?.duplicated}
	<p class="banner ok">Encuesta duplicada.</p>
{:else if form?.deleted}
	<p class="banner ok">Encuesta eliminada.</p>
{/if}

{#if showCreate}
	<form class="create-card" method="POST" action="?/create" use:enhance>
		<h2>Nueva encuesta</h2>

		{#if data.teams.length === 0}
			<p class="hint">No perteneces a ningún equipo todavía. Crea o únete a un equipo antes de crear encuestas.</p>
		{/if}

		<label>
			Título
			<input type="text" name="title" required placeholder="Encuesta de satisfacción" />
		</label>

		<label>
			Descripción
			<textarea name="description" rows="2" placeholder="Opcional"></textarea>
		</label>

		<label>
			Equipo
			<select name="team_id" required>
				<option value="" disabled selected>Selecciona un equipo</option>
				{#each data.teams as team (team.id)}
					<option value={team.id}>{team.name}</option>
				{/each}
			</select>
		</label>

		<label>
			Nivel de anonimato
			<select name="anonymity_level">
				<option value="none">Ninguno</option>
				<option value="partial">Parcial</option>
				<option value="full">Total</option>
			</select>
		</label>

		<label class="check">
			<input type="checkbox" name="allow_revisit" />
			Permitir volver a responder
		</label>
		<label class="check">
			<input type="checkbox" name="optional_registration" />
			Registro opcional
		</label>

		<button class="primary" type="submit" disabled={data.teams.length === 0}>Crear</button>
	</form>
{/if}

{#if data.surveys.length === 0}
	<p class="empty">Aún no hay encuestas.</p>
{:else}
	<table>
		<thead>
			<tr>
				<th>Título</th>
				<th>Estado</th>
				<th>Anonimato</th>
				<th>Creada</th>
				<th>Dueño</th>
				<th class="actions-col">Acciones</th>
			</tr>
		</thead>
		<tbody>
			{#each data.surveys as survey (survey.id)}
				<tr>
					<td><a href="/admin/surveys/{survey.id}">{survey.title}</a></td>
					<td><span class="badge">{STATUS_LABELS[survey.status] ?? survey.status}</span></td>
					<td>{ANONYMITY_LABELS[survey.anonymity_level]}</td>
					<td>{fmtDate(survey.created_at)}</td>
					<td>{survey.owner_name ?? '—'}</td>
					<td class="actions">
						<a class="link-btn" href="/admin/surveys/{survey.id}">Editar</a>

						<form method="POST" action="?/duplicate" use:enhance>
							<input type="hidden" name="id" value={survey.id} />
							<button type="submit">Duplicar</button>
						</form>

						<form
							method="POST"
							action="?/delete"
							use:enhance
							onsubmit={(e) => {
								if (!confirm(`¿Eliminar "${survey.title}"?`)) e.preventDefault();
							}}
						>
							<input type="hidden" name="id" value={survey.id} />
							<button type="submit" class="danger">Eliminar</button>
						</form>
					</td>
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
		margin-bottom: 1rem;
	}

	h1 {
		font-size: 1.5rem;
		color: var(--blue-900);
		margin: 0;
	}

	.banner {
		padding: 0.625rem 0.75rem;
		border-radius: var(--radius-sm);
		font-size: 0.875rem;
		margin-bottom: 1rem;
	}
	.banner.error {
		background: var(--danger-lt);
		border-left: 3px solid var(--danger);
		color: var(--danger);
	}
	.banner.ok {
		background: var(--success-lt);
		border-left: 3px solid var(--success);
		color: var(--success);
	}

	.create-card {
		background: var(--white);
		border: 1px solid var(--border);
		border-top: 5px solid var(--blue-900);
		border-radius: var(--radius-sm);
		padding: 1.25rem;
		margin-bottom: 1.5rem;
		max-width: 560px;
	}
	.create-card h2 {
		font-size: 1.1rem;
		margin: 0 0 1rem;
		color: var(--blue-900);
	}

	label {
		display: block;
		margin-bottom: 0.875rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--blue-900);
	}
	label.check {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-weight: 400;
	}

	.hint {
		background: var(--warning-lt);
		border-left: 3px solid var(--warning);
		color: var(--warning);
		font-size: 0.85rem;
		padding: 0.5rem 0.75rem;
		border-radius: var(--radius-sm);
		margin-bottom: 0.875rem;
	}

	input[type='text'],
	textarea,
	select {
		display: block;
		width: 100%;
		margin-top: 0.25rem;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 0.95rem;
		box-sizing: border-box;
		font-family: inherit;
	}

	button {
		padding: 0.4rem 0.9rem;
		border: 1px solid var(--border);
		background: white;
		border-radius: var(--radius-pill);
		font-family: var(--font-display);
		font-size: 0.8rem;
		font-weight: 600;
		cursor: pointer;
		color: var(--blue-900);
	}
	button:hover {
		border-color: var(--blue-900);
		background: var(--blue-50);
	}
	button.primary {
		background: var(--blue-900);
		border-color: var(--blue-900);
		color: white;
	}
	button.primary:hover {
		background: var(--blue-700);
		border-color: var(--blue-700);
	}
	button.primary:disabled {
		background: var(--blue-300);
		border-color: var(--blue-300);
		cursor: not-allowed;
	}
	button.danger {
		color: var(--danger);
		border-color: var(--danger);
	}
	button.danger:hover {
		background: var(--danger-lt);
		border-color: var(--danger);
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}
	th,
	td {
		text-align: left;
		padding: 0.7rem 0.75rem;
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

	.actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.actions form {
		display: inline;
	}
	.link-btn {
		padding: 0.4rem 0.9rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-pill);
		font-family: var(--font-display);
		font-size: 0.8rem;
		color: var(--blue-900) !important;
		text-decoration: none !important;
		font-weight: 600 !important;
	}
	.link-btn:hover {
		border-color: var(--blue-900);
		background: var(--blue-50);
	}

	.empty {
		color: var(--muted);
		font-size: 0.95rem;
		padding: 2rem 0;
	}
</style>
