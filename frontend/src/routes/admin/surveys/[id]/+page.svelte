<script lang="ts">
	import { enhance } from '$app/forms';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);

	function fmtDate(iso: string): string {
		return new Date(iso).toLocaleString();
	}
</script>

<svelte:head>
	<title>Editar: {survey.title} — AI Surveys</title>
</svelte:head>

<p class="breadcrumb"><a href="/admin/surveys">← Encuestas</a></p>

<div class="header">
	<h1>Editar encuesta</h1>
	<span class="badge">{survey.status}</span>
</div>

{#if form?.error}
	<p class="banner error" role="alert">{form.error}</p>
{:else if form?.updated}
	<p class="banner ok">Cambios guardados.</p>
{/if}

<form class="card" method="POST" action="?/update" use:enhance>
	<label>
		Título
		<input type="text" name="title" value={survey.title} required />
	</label>

	<label>
		Descripción
		<textarea name="description" rows="3">{survey.description ?? ''}</textarea>
	</label>

	<label>
		Nivel de anonimato
		<select name="anonymity_level" value={survey.anonymity_level}>
			<option value="none">Ninguno</option>
			<option value="partial">Parcial</option>
			<option value="full">Total</option>
		</select>
		<span class="sub">Se bloquea una vez que llega la primera respuesta.</span>
	</label>

	<label class="check">
		<input type="checkbox" name="allow_revisit" checked={survey.allow_revisit} />
		Permitir volver a responder
	</label>
	<label class="check">
		<input type="checkbox" name="optional_registration" checked={survey.optional_registration} />
		Registro opcional
	</label>

	<button class="primary" type="submit">Guardar cambios</button>
</form>

<div class="meta">
	<span>Equipo: <strong>{survey.team_name ?? '—'}</strong></span>
	<span>Dueño: <strong>{survey.owner_name ?? '—'}</strong></span>
	<span>Creada: {fmtDate(survey.created_at)}</span>
	<span>Actualizada: {fmtDate(survey.updated_at)}</span>
</div>

<div class="danger-zone">
	<form method="POST" action="?/duplicate" use:enhance>
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
		<button type="submit" class="danger">Eliminar</button>
	</form>
</div>

<style>
	.breadcrumb {
		margin: 0 0 1rem;
		font-size: 0.875rem;
	}
	.breadcrumb a {
		color: #2563eb;
		text-decoration: none;
	}

	.header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}
	h1 {
		font-size: 1.5rem;
		color: #0f172a;
		margin: 0;
	}
	.badge {
		display: inline-block;
		padding: 0.15rem 0.6rem;
		background: #f1f5f9;
		border-radius: 999px;
		font-size: 0.8rem;
		color: #475569;
	}

	.banner {
		padding: 0.625rem 0.75rem;
		border-radius: 6px;
		font-size: 0.875rem;
		margin-bottom: 1rem;
		max-width: 560px;
	}
	.banner.error {
		background: #fef2f2;
		border: 1px solid #fca5a5;
		color: #dc2626;
	}
	.banner.ok {
		background: #f0fdf4;
		border: 1px solid #86efac;
		color: #16a34a;
	}

	.card {
		background: #f8fafc;
		border: 1px solid #e2e8f0;
		border-radius: 8px;
		padding: 1.25rem;
		max-width: 560px;
	}

	label {
		display: block;
		margin-bottom: 0.875rem;
		font-size: 0.875rem;
		font-weight: 500;
		color: #374151;
	}
	label.check {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-weight: 400;
	}
	.sub {
		display: block;
		font-weight: 400;
		font-size: 0.78rem;
		color: #94a3b8;
		margin-top: 0.25rem;
	}

	input[type='text'],
	textarea,
	select {
		display: block;
		width: 100%;
		margin-top: 0.25rem;
		padding: 0.5rem 0.75rem;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		font-size: 0.95rem;
		box-sizing: border-box;
		font-family: inherit;
	}

	button {
		padding: 0.45rem 0.9rem;
		border: 1px solid #d1d5db;
		background: white;
		border-radius: 6px;
		font-size: 0.875rem;
		cursor: pointer;
		color: #374151;
	}
	button:hover {
		border-color: #94a3b8;
	}
	button.primary {
		background: #2563eb;
		border-color: #2563eb;
		color: white;
	}
	button.primary:hover {
		background: #1d4ed8;
	}
	button.danger {
		color: #dc2626;
		border-color: #fca5a5;
	}
	button.danger:hover {
		background: #fef2f2;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 1.25rem;
		margin: 1.25rem 0;
		font-size: 0.82rem;
		color: #64748b;
		max-width: 560px;
	}

	.danger-zone {
		display: flex;
		gap: 0.75rem;
		padding-top: 1rem;
		border-top: 1px solid #e5e7eb;
		max-width: 560px;
	}
	.danger-zone form {
		display: inline;
	}
</style>
