<script lang="ts">
	import type { PageData, ActionData } from './$types';
	import { enhance } from '$app/forms';
	import { LANGUAGE_LABELS, STATUS_LABELS } from '$lib/types';
	import type { LanguageCode } from '$lib/types';
	import { getPermissions } from '$lib/permissions';
	import QuestionAnalysisCard from '$lib/components/analysis/QuestionAnalysisCard.svelte';
	import EmptyState from '$lib/components/shared/EmptyState.svelte';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);
	const analysis = $derived(data.analysis);
	const stats = $derived(analysis.stats);
	const perms = $derived(getPermissions(data.user?.role ?? 'alumno'));

	let retrying = $state(false);

	// En 'analysing' el job sigue corriendo: la página es legible, pero algunas
	// preguntas todavía no tienen resultado.
	const running = $derived(analysis.status === 'analysing');
	// 'failed' es un estado terminal: el job murió a mitad de camino (proveedor
	// caído, JSON inválido, timeout) y no va a avanzar solo — hace falta
	// reintentar (ver AnalysisService.AnalyseSurvey / RetryAnalysis).
	const failed = $derived(analysis.status === 'failed');

	function formatDate(dateStr: string) {
		return new Date(dateStr).toLocaleDateString('es-MX', {
			year: 'numeric',
			month: 'long',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function formatDuration(seconds: number | null): string {
		if (seconds === null) return '—';
		const mins = Math.floor(seconds / 60);
		const secs = Math.round(seconds % 60);
		return mins > 0 ? `${mins} min ${secs} s` : `${secs} s`;
	}

	const completionPct = $derived(Math.round(stats.completion_rate * 100));

	function languageLabel(code: string): string {
		return LANGUAGE_LABELS[code as LanguageCode] ?? code;
	}
</script>

<svelte:head><title>Análisis: {survey.title} — AI Surveys</title></svelte:head>

<div class="page">
	<div class="page-hero">
		<div class="hero-inner">
			<div>
				<a href="/dashboard/surveys/{survey.id}/lifecycle" class="back-link">← {survey.title}</a>
				<h1 class="page-title">Análisis de resultados</h1>
				<p class="page-sub">
					{#if analysis.analysed_at}
						Analizada el {formatDate(analysis.analysed_at)}
					{:else}
						El análisis todavía no produce resultados.
					{/if}
				</p>
			</div>
			<div class="hero-actions">
				<span class="status-badge">{STATUS_LABELS[analysis.status] ?? analysis.status}</span>
				<a href="/dashboard/surveys/{survey.id}/responses" class="hero-link">Ver respuestas →</a>
			</div>
		</div>
	</div>

	<div class="page-body">
		{#if running}
			<p class="notice">
				El análisis está en curso. Las preguntas que falten aparecerán en cuanto el motor
				termine — recarga en unos momentos.
			</p>
		{/if}

		{#if failed}
			<div class="notice notice--error">
				<p>
					El análisis no pudo completarse{#if analysis.questions.some((q) => q.analysed_at)}, aunque
						algunas preguntas de abajo sí alcanzaron a analizarse{/if}.
				</p>
				{#if form?.retryError}<p class="retry-error">{form.retryError}</p>{/if}
				{#if perms.canEditSurvey}
					<form
						method="POST"
						action="?/retry"
						use:enhance={() => {
							retrying = true;
							return async ({ update }) => {
								retrying = false;
								await update();
							};
						}}
					>
						<button type="submit" class="btn-retry" disabled={retrying}>
							{retrying ? 'Reintentando…' : 'Reintentar análisis'}
						</button>
					</form>
				{/if}
			</div>
		{/if}

		<!-- Stats a nivel encuesta -->
		<div class="stats-grid">
			<div class="stat">
				<span class="stat-label">Respuestas</span>
				<span class="stat-value">{stats.response_count}</span>
			</div>
			<div class="stat">
				<span class="stat-label">Completadas</span>
				<span class="stat-value">{stats.completed_count}</span>
			</div>
			<div class="stat">
				<span class="stat-label">Tasa de completado</span>
				<span class="stat-value">{completionPct}<span class="unit">%</span></span>
			</div>
			<div class="stat">
				<span class="stat-label">Duración promedio</span>
				<span class="stat-value stat-value--sm">{formatDuration(stats.avg_duration_seconds)}</span>
			</div>
			<div class="stat stat--wide">
				<span class="stat-label">Idiomas</span>
				{#if stats.language_distribution.length > 0}
					<ul class="langs">
						{#each stats.language_distribution as lang (lang.language)}
							<li>
								{languageLabel(lang.language)}
								<strong>{lang.count}</strong>
							</li>
						{/each}
					</ul>
				{:else}
					<span class="stat-value stat-value--sm">—</span>
				{/if}
			</div>
		</div>

		<!-- Una sección por pregunta, en orden de encuesta -->
		{#if analysis.questions.length > 0}
			<div class="questions">
				{#each analysis.questions as question, i (question.question_id)}
					<QuestionAnalysisCard {question} position={i + 1} />
				{/each}
			</div>
		{:else}
			<EmptyState
				title="Esta encuesta no tiene preguntas"
				description={survey.mode === 'prompt_only'
					? 'En Modo B la IA conduce la conversación libremente, así que no hay secciones por pregunta que analizar.'
					: 'Sin preguntas no hay secciones por pregunta que analizar.'}
			/>
		{/if}
	</div>
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		width: 100%;
		min-height: calc(100vh - 60px);
	}

	.page-hero {
		background: var(--blue-700);
		padding: 2rem 3rem;
	}

	.hero-actions {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		flex-wrap: wrap;
	}

	.hero-link {
		font-size: 0.8rem;
		color: rgba(255, 255, 255, 0.85);
		font-weight: 600;
		text-decoration: none;
	}
	.hero-link:hover {
		color: white;
		text-decoration: underline;
	}

	.hero-inner {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		flex-wrap: wrap;
		max-width: 1100px;
		margin: 0 auto;
	}

	.back-link {
		font-size: 0.8rem;
		color: rgba(255, 255, 255, 0.65);
		text-decoration: none;
		display: block;
		margin-bottom: 0.5rem;
		transition: color 0.15s;
	}

	.back-link:hover {
		color: white;
	}

	.page-title {
		font-size: 1.6rem;
		font-weight: 700;
		letter-spacing: -0.03em;
		color: white;
		margin: 0 0 0.4rem;
	}
	.page-sub {
		font-size: 0.8rem;
		color: rgba(255, 255, 255, 0.65);
		margin: 0;
	}

	.status-badge {
		font-size: 0.75rem;
		font-weight: 700;
		padding: 0.3rem 0.9rem;
		border-radius: var(--radius-pill);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		white-space: nowrap;
		background: rgba(255, 255, 255, 0.15);
		border: 1px solid rgba(255, 255, 255, 0.3);
		color: white;
	}

	.page-body {
		padding: 1.75rem 3rem;
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
		max-width: 1100px;
		margin: 0 auto;
		width: 100%;
	}

	.notice {
		background: var(--blue-50);
		border: 1.5px solid var(--blue-200);
		border-radius: var(--radius-md);
		padding: 0.7rem 1rem;
		font-size: 0.85rem;
		color: var(--blue-800);
		margin: 0;
		line-height: 1.5;
	}

	.notice--error {
		background: var(--danger-lt);
		border-color: var(--danger);
		color: var(--danger);
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	.notice--error p {
		margin: 0;
	}

	.retry-error {
		font-weight: 600;
	}

	.btn-retry {
		align-self: flex-start;
		background: var(--danger);
		color: white;
		border: none;
		border-radius: var(--radius-md);
		padding: 0.45rem 1rem;
		font-size: 0.82rem;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.15s;
	}

	.btn-retry:hover:not(:disabled) {
		background: var(--danger);
	}
	.btn-retry:disabled {
		opacity: 0.6;
		cursor: default;
	}

	/* ── Stats ────────────────────────────────────────────────────────── */
	.stats-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 1rem;
		background: var(--white);
		border: 1.5px solid var(--blue-100);
		border-radius: var(--radius-lg);
		padding: 1.5rem 1.75rem;
		box-shadow: var(--shadow-sm);
	}

	.stat {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.stat--wide {
		grid-column: 1 / -1;
		padding-top: 1rem;
		border-top: 1.5px solid var(--blue-50);
	}

	.stat-label {
		font-size: 0.72rem;
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.stat-value {
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--text);
		letter-spacing: -0.02em;
		line-height: 1.2;
	}
	.stat-value--sm {
		font-size: 1rem;
		font-weight: 600;
	}
	.unit {
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--muted);
		margin-left: 0.1rem;
	}

	.langs {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		list-style: none;
		margin: 0.15rem 0 0;
		padding: 0;
	}

	.langs li {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.8rem;
		color: var(--muted);
		background: var(--blue-50);
		border: 1px solid var(--blue-200);
		border-radius: var(--radius-pill);
		padding: 0.15rem 0.7rem;
	}

	.langs strong {
		color: var(--blue-700);
		font-weight: 700;
	}

	.questions {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	@media (max-width: 700px) {
		.page-hero {
			padding: 1.75rem 1.5rem;
		}
		.page-body {
			padding: 1.25rem 1.5rem;
		}
		.stats-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}
</style>
