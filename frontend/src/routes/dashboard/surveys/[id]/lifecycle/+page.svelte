<script lang="ts">
	import { enhance } from '$app/forms';
	import { STATUS_LABELS, hasAnalysis } from '$lib/types';
	import { getPermissions } from '$lib/permissions';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);
	const stats = $derived(data.stats);
	const perms = $derived(getPermissions(data.user?.role ?? 'alumno'));
	const canSeeAnalysis = $derived(perms.canViewResults && hasAnalysis(survey.status));
	const publicPath = $derived(`/s/${survey.public_token}`);

	function formatDuration(seconds: number | null | undefined): string {
		if (seconds === null || seconds === undefined) return '—';
		const mins = Math.floor(seconds / 60);
		const secs = Math.round(seconds % 60);
		return mins > 0 ? `${mins} min ${secs} s` : `${secs} s`;
	}

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
	{#if perms.canViewResults}
		<a href="/dashboard/surveys/{survey.id}/analysis">Resultados</a>
	{/if}
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
		{#if survey.status === 'failed' || survey.status === 'complete'}
			<form
				method="POST"
				action="?/retry"
				use:enhance
				onsubmit={(e) => {
					const msg =
						survey.status === 'complete'
							? '¿Volver a analizar esta encuesta? Se sobrescribirán los resultados actuales.'
							: '¿Reintentar el análisis? El intento anterior no pudo completarse.';
					if (!confirm(msg)) e.preventDefault();
				}}
			>
				<button class="primary" type="submit">
					{survey.status === 'complete' ? 'Volver a analizar' : 'Reintentar análisis'}
				</button>
			</form>
		{/if}
	</div>
</div>

<!-- Resultados (#16) -->
{#if perms.canViewResults && stats}
	<div class="card">
		<h2 class="card-title">
			Resultados
			<a href="/dashboard/surveys/{survey.id}/responses" class="link-action">Ver respuestas →</a>
			{#if canSeeAnalysis}
				<a href="/dashboard/surveys/{survey.id}/analysis" class="link-action">Ver análisis →</a>
			{/if}
		</h2>
		<div class="stats-row">
			<div class="ms-item">
				<span class="ms-label">Respuestas</span>
				<span class="stat-value">{stats.response_count}</span>
			</div>
			<div class="ms-item">
				<span class="ms-label">Completadas</span>
				<span class="stat-value">{stats.completed_count}</span>
			</div>
			<div class="ms-item">
				<span class="ms-label">Tasa de completado</span>
				<span class="stat-value">{Math.round(stats.completion_rate * 100)}%</span>
			</div>
			<div class="ms-item">
				<span class="ms-label">Duración promedio</span>
				<span class="stat-value stat-value--sm">{formatDuration(stats.avg_duration_seconds)}</span>
			</div>
		</div>

		{#if survey.anonymity_level === 'full'}
			<p class="hint-inline stats-hint">
				En una encuesta completamente anónima, «Tasa de completado» mide sesiones
				(abre/no abre), no personas: cada visita crea una respuesta nueva, así que
				reintentos y abandonos la inflan. Declara un tope de respuestas arriba para
				ver cobertura real.
			</p>
		{/if}

		{#if stats.expected_responses !== null}
			<div class="coverage-row">
				<div class="ms-item">
					<span class="ms-label">Cobertura esperada</span>
					<span class="stat-value">{Math.round((stats.coverage_rate ?? 0) * 100)}%</span>
				</div>
				<div class="ms-item">
					<span class="ms-label">Faltan por responder</span>
					<span class="stat-value" class:stat-value--warn={(stats.missing_responses ?? 0) > 0}>
						{stats.missing_responses}
					</span>
				</div>
				<div class="ms-item">
					<span class="ms-label">Esperadas en total</span>
					<span class="stat-value stat-value--sm">{stats.expected_responses}</span>
				</div>
			</div>
		{/if}

		{#if !canSeeAnalysis}
			<p class="hint-inline stats-hint">El análisis por pregunta se genera al finalizar la encuesta.</p>
		{/if}
	</div>
{/if}

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

	.card-title {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.link-action {
		font-size: 0.8rem;
		color: var(--blue-600);
		text-decoration: none;
		font-weight: 600;
		margin-left: auto;
	}
	.link-action:hover {
		color: var(--blue-800);
		text-decoration: underline;
	}
	.link-action + .link-action {
		margin-left: 0;
	}

	.lifecycle-actions {
		display: flex;
		gap: 0.6rem;
		flex-wrap: wrap;
	}
	.lifecycle-actions form {
		display: inline;
	}

	.stats-row {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 0.75rem 1.5rem;
	}

	.coverage-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.75rem 1.5rem;
		margin-top: 1.25rem;
		padding-top: 1.25rem;
		border-top: 1.5px solid var(--blue-50);
	}

	.ms-item {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.ms-label {
		font-size: 0.72rem;
		color: var(--muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.stat-value {
		font-size: 1.4rem;
		font-weight: 700;
		color: var(--text);
		letter-spacing: -0.02em;
		line-height: 1.2;
	}
	.stat-value--sm {
		font-size: 1rem;
		font-weight: 600;
	}
	.stat-value--warn {
		color: #b45309;
	}

	.stats-hint {
		padding-top: 1rem;
	}

	@media (max-width: 700px) {
		.stats-row {
			grid-template-columns: repeat(2, 1fr);
		}
		.coverage-row {
			grid-template-columns: repeat(2, 1fr);
		}
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
