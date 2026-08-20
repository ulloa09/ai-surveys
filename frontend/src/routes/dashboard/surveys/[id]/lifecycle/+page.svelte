<script lang="ts">
	import { enhance } from '$app/forms';
	import { STATUS_LABELS } from '$lib/types';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);
	const publicPath = $derived(`/s/${survey.public_token}`);

	function toLocalInputValue(iso: string | null): string {
		if (!iso) return '';
		const d = new Date(iso);
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}

	let opensAt = $state(toLocalInputValue(survey.opens_at));
	let closesAt = $state(toLocalInputValue(survey.closes_at));
	let responseCap = $state(survey.response_cap ?? '');

	$effect(() => {
		opensAt = toLocalInputValue(survey.opens_at);
		closesAt = toLocalInputValue(survey.closes_at);
		responseCap = survey.response_cap ?? '';
	});
</script>

<svelte:head>
	<title>Ciclo de vida: {survey.title} — AI Surveys</title>
</svelte:head>

<p class="breadcrumb"><a href="/dashboard/surveys">← Encuestas</a></p>

<div class="header">
	<h1>{survey.title}</h1>
	<span class="badge badge--{survey.status}">{STATUS_LABELS[survey.status] ?? survey.status}</span>
</div>

<nav class="subnav">
	<a href="/dashboard/surveys/{survey.id}">Modo &amp; IA</a>
	<a href="/dashboard/surveys/{survey.id}/questions">Preguntas</a>
	<a href="/dashboard/surveys/{survey.id}/lifecycle" class="active">Ciclo de vida</a>
	<a href="/admin/surveys/{survey.id}">Configuración básica</a>
</nav>

{#if form?.lifecycleError}
	<p class="banner error" role="alert">{form.lifecycleError}</p>
{:else if form?.lifecycleUpdated}
	<p class="banner ok">Estado actualizado.</p>
{/if}

<div class="card">
	<h2>Estado</h2>
	<div class="lifecycle-actions">
		{#if survey.status === 'draft'}
			<form method="POST" action="?/activate" use:enhance>
				<button class="primary" type="submit">Activar</button>
			</form>
		{/if}
		{#if survey.status === 'open'}
			<form
				method="POST"
				action="?/close"
				use:enhance
				onsubmit={(e) => {
					if (!confirm('¿Cerrar esta encuesta? Dejará de aceptar respuestas.')) e.preventDefault();
				}}
			>
				<button class="primary" type="submit">Cerrar</button>
			</form>
		{/if}
		{#if survey.status === 'closed'}
			<form method="POST" action="?/reopen" use:enhance>
				<button class="primary" type="submit">Reabrir</button>
			</form>
			<form
				method="POST"
				action="?/archive"
				use:enhance
				onsubmit={(e) => {
					if (!confirm('¿Archivar esta encuesta? Se oculta del listado por defecto.')) e.preventDefault();
				}}
			>
				<button class="danger" type="submit">Archivar</button>
			</form>
		{/if}
	</div>
</div>

{#if form?.scheduleError}
	<p class="banner error" role="alert">{form.scheduleError}</p>
{:else if form?.scheduleUpdated}
	<p class="banner ok">Programación guardada.</p>
{/if}

<form class="card" method="POST" action="?/updateSchedule" use:enhance>
	<h2>Programación y tope de respuestas</h2>
	<label>
		Apertura automática
		<input type="datetime-local" name="opens_at" bind:value={opensAt} />
	</label>
	<label>
		Cierre automático
		<input type="datetime-local" name="closes_at" bind:value={closesAt} />
	</label>
	<label>
		Tope de respuestas
		<input type="number" name="response_cap" min="1" bind:value={responseCap} placeholder="Sin límite" />
	</label>
	<p class="hint-inline">
		La encuesta se abre/cierra sola según estas fechas (revisado cada minuto), y se cierra
		automáticamente al alcanzar el tope de respuestas si se indica uno.
	</p>
	<button class="primary" type="submit">Guardar programación</button>
</form>

{#if survey.qr_png_url}
	<div class="card">
		<h2>Acceso público</h2>
		<p class="public-link">{publicPath}</p>
		<div class="qr-row">
			<img class="qr-preview" src={survey.qr_png_url} alt="Código QR de la encuesta" />
			<div class="qr-links">
				<a href={survey.qr_png_url} download="encuesta-{survey.id}.png">Descargar PNG</a>
				{#if survey.qr_svg_url}
					<a href={survey.qr_svg_url} download="encuesta-{survey.id}.svg">Descargar SVG</a>
				{/if}
			</div>
		</div>
	</div>
{:else}
	<div class="card">
		<h2>Acceso público</h2>
		<p class="hint-inline">El link y el código QR se generan la primera vez que actives la encuesta.</p>
	</div>
{/if}

<style>
	.breadcrumb {
		margin: 0 0 1rem;
		font-size: 0.875rem;
	}
	.breadcrumb a {
		color: var(--blue-600);
		text-decoration: none;
	}

	.header {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.75rem;
	}
	h1 {
		font-size: 1.5rem;
		color: var(--blue-900);
		margin: 0;
	}
	.badge {
		display: inline-block;
		padding: 0.15rem 0.6rem;
		background: var(--blue-50);
		border: 1px solid var(--border);
		border-radius: var(--radius-pill);
		font-size: 0.75rem;
		color: var(--muted);
	}
	.badge--open {
		background: var(--success-lt);
		color: var(--success);
		border-color: var(--success);
	}
	.badge--closed {
		background: var(--warning-lt);
		color: var(--warning);
		border-color: var(--warning);
	}

	.subnav {
		display: flex;
		gap: 1.25rem;
		margin-bottom: 1.5rem;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.6rem;
	}
	.subnav a {
		font-family: var(--font-display);
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--muted);
		text-decoration: none;
	}
	.subnav a.active {
		color: var(--blue-900);
	}
	.subnav a:hover {
		color: var(--blue-600);
	}

	.banner {
		padding: 0.625rem 0.75rem;
		border-radius: var(--radius-sm);
		font-size: 0.875rem;
		margin-bottom: 1rem;
		max-width: 640px;
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

	.card {
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 1.5rem;
		max-width: 640px;
		margin-bottom: 1.25rem;
	}

	h2 {
		font-size: 1rem;
		color: var(--blue-900);
		margin: 0 0 0.9rem;
	}

	.lifecycle-actions {
		display: flex;
		gap: 0.6rem;
	}
	.lifecycle-actions form {
		display: inline;
	}

	label {
		display: block;
		margin-bottom: 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--blue-900);
	}

	input[type='datetime-local'],
	input[type='number'] {
		display: block;
		width: 100%;
		max-width: 280px;
		margin-top: 0.35rem;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 0.9rem;
		box-sizing: border-box;
		font-family: inherit;
	}

	.hint-inline {
		font-size: 0.78rem;
		color: var(--muted);
		margin: 0 0 1rem;
	}

	button.primary {
		background: var(--blue-900);
		border: 1px solid var(--blue-900);
		border-radius: var(--radius-pill);
		color: #fff;
		font-family: var(--font-display);
		font-size: 0.85rem;
		font-weight: 600;
		padding: 0.55rem 1.25rem;
		cursor: pointer;
	}
	button.primary:hover {
		background: var(--blue-700);
		border-color: var(--blue-700);
	}
	button.danger {
		background: var(--white);
		border: 1px solid var(--danger);
		color: var(--danger);
		border-radius: var(--radius-pill);
		font-family: var(--font-display);
		font-size: 0.85rem;
		font-weight: 600;
		padding: 0.55rem 1.25rem;
		cursor: pointer;
	}
	button.danger:hover {
		background: var(--danger-lt);
	}

	.public-link {
		font-family: monospace;
		font-size: 0.85rem;
		color: var(--blue-700);
		background: var(--blue-50);
		border-radius: var(--radius-sm);
		padding: 0.5rem 0.75rem;
		margin: 0 0 1rem;
	}

	.qr-row {
		display: flex;
		align-items: center;
		gap: 1.25rem;
	}
	.qr-preview {
		width: 120px;
		height: 120px;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
	}
	.qr-links {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.qr-links a {
		color: var(--blue-600);
		font-size: 0.85rem;
		text-decoration: none;
		font-weight: 600;
	}
	.qr-links a:hover {
		text-decoration: underline;
	}
</style>
