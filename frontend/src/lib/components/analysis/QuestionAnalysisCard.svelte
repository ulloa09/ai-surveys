<script lang="ts">
	import type { QuestionAnalysis } from '$lib/types';
	import { QUESTION_TYPE_LABELS } from '$lib/types';
	import SentimentBar from './SentimentBar.svelte';
	import TopicClusters from './TopicClusters.svelte';
	import AnswerDistribution from './AnswerDistribution.svelte';

	let { question, position }: { question: QuestionAnalysis; position: number } = $props();

	// Mientras la encuesta está en 'analysing' el job va escribiendo pregunta por
	// pregunta: las que todavía no tienen resultado se muestran igual, en espera,
	// en vez de desaparecer de la lista.
	const pending = $derived(question.analysed_at === null);

	// Las preguntas abiertas se agregan por temas y respuestas atípicas; las
	// estructuradas, por distribución de opciones. Ver QuestionAnalysis: el
	// backend solo llena el agregado que corresponde al tipo.
	const openEnded = $derived(question.type === 'open_ended');

	const sentimentLabels: Record<string, string> = {
		positive: 'Positivo',
		neutral: 'Neutral',
		negative: 'Negativo'
	};
</script>

<section class="card" class:card--pending={pending}>
	<header class="q-header">
		<span class="q-num">{position}</span>
		<div class="q-heading">
			<h2 class="q-text">{question.text}</h2>
			<div class="q-meta">
				<span class="type-badge">{QUESTION_TYPE_LABELS[question.type] ?? question.type}</span>
				<span class="answer-count">
					{question.answer_count}
					{question.answer_count === 1 ? 'respuesta' : 'respuestas'}
				</span>
			</div>
		</div>
	</header>

	{#if pending}
		<p class="pending-msg">
			Esta pregunta todavía no se analiza. El resultado aparecerá al terminar el análisis.
		</p>
	{:else}
		<div class="block">
			<h3 class="block-title">Resumen</h3>
			{#if question.summary_text}
				<p class="summary">{question.summary_text}</p>
			{:else}
				<p class="no-data">Sin respuestas que resumir.</p>
			{/if}
		</div>

		<div class="block-grid">
			<div class="block">
				<h3 class="block-title">Sentimiento</h3>
				<SentimentBar distribution={question.sentiment_distribution} />
			</div>

			<div class="block">
				{#if openEnded}
					<h3 class="block-title">Temas</h3>
					<TopicClusters clusters={question.topic_clusters} />
				{:else}
					<h3 class="block-title">Distribución</h3>
					<AnswerDistribution distribution={question.answer_distribution} />
				{/if}
			</div>
		</div>

		{#if question.outliers.length > 0}
			<div class="block">
				<h3 class="block-title">
					Respuestas atípicas
					<span class="outlier-count">{question.outliers.length}</span>
				</h3>
				<ul class="outliers">
					{#each question.outliers as outlier (outlier.response_id)}
						<li class="outlier">
							<span class="flag" aria-hidden="true"></span>
							<div class="outlier-body">
								<p class="outlier-text">{outlier.raw_value}</p>
								{#if outlier.sentiment_label}
									<span class="outlier-sentiment">
										{sentimentLabels[outlier.sentiment_label] ?? outlier.sentiment_label}
									</span>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	{/if}
</section>

<style>
	.card {
		background: var(--white);
		border: 1.5px solid var(--blue-100);
		border-radius: var(--radius-lg);
		padding: 1.75rem;
		box-shadow: var(--shadow-sm);
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.card--pending {
		border-style: dashed;
		box-shadow: none;
	}

	/* ── Header ───────────────────────────────────────────────────────── */
	.q-header {
		display: flex;
		align-items: flex-start;
		gap: 0.875rem;
	}

	.q-num {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		background: var(--blue-900);
		color: white;
		font-size: 0.775rem;
		font-weight: 700;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.q-heading {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}

	.q-text {
		font-size: 1rem;
		font-weight: 700;
		color: var(--text);
		letter-spacing: -0.01em;
		line-height: 1.4;
		margin: 0;
	}

	.q-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.type-badge {
		font-size: 0.65rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--blue-600);
		background: var(--blue-50);
		border: 1px solid var(--blue-200);
		border-radius: var(--radius-pill);
		padding: 0.15rem 0.6rem;
	}

	.answer-count {
		font-size: 0.775rem;
		color: var(--muted);
	}

	/* ── Blocks ───────────────────────────────────────────────────────── */
	.block {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	.block-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.75rem;
	}

	.block-title {
		font-size: 0.72rem;
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		margin: 0;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.summary {
		font-size: 0.9rem;
		color: var(--text);
		line-height: 1.65;
		margin: 0;
	}

	.no-data,
	.pending-msg {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
	}

	/* ── Outliers ─────────────────────────────────────────────────────── */
	.outlier-count {
		background: var(--danger-lt);
		border: 1px solid var(--danger);
		color: #be123c;
		border-radius: var(--radius-pill);
		padding: 0 0.45rem;
		font-size: 0.7rem;
	}

	.outliers {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	/* Las atípicas se distinguen de las respuestas normales por el borde
	   ámbar, el fondo tintado y la bandera — no solo por la posición. */
	.outlier {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		background: var(--warning-lt);
		border: 1.5px solid var(--warning);
		border-left: 3px solid #d97706;
		border-radius: var(--radius-md);
		padding: 0.75rem 1rem;
	}

	.flag {
		width: 4px;
		align-self: stretch;
		background: var(--warning);
		flex-shrink: 0;
	}

	.outlier-body {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		min-width: 0;
	}

	.outlier-text {
		font-size: 0.85rem;
		color: var(--warning);
		line-height: 1.55;
		margin: 0;
		overflow-wrap: anywhere;
	}

	.outlier-sentiment {
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--warning);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	@media (max-width: 700px) {
		.block-grid {
			grid-template-columns: 1fr;
			gap: 1.25rem;
		}
	}
</style>
