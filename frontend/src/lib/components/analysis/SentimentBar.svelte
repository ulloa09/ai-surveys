<script lang="ts">
	import type { SentimentDistribution } from '$lib/types';

	let { distribution }: { distribution: SentimentDistribution } = $props();

	// Una pregunta sin respuestas analizadas trae la distribución en ceros: la
	// barra no se dibuja, porque un 0/0/0 se vería igual que "todo neutral".
	const total = $derived(distribution.positive + distribution.neutral + distribution.negative);
	const hasData = $derived(total > 0);

	// Cada segmento se redondea por separado para la etiqueta, pero el ancho usa
	// la fracción cruda — así la barra siempre suma 100% aunque las etiquetas no.
	function pct(value: number): number {
		return Math.round(value * 100);
	}

	const segments = $derived([
		{ key: 'positive', label: 'Positivo', value: distribution.positive },
		{ key: 'neutral', label: 'Neutral', value: distribution.neutral },
		{ key: 'negative', label: 'Negativo', value: distribution.negative }
	]);

	const summary = $derived(segments.map((s) => `${s.label} ${pct(s.value)}%`).join(', '));
</script>

{#if hasData}
	<div class="sentiment">
		<div class="bar" role="img" aria-label="Distribución de sentimiento: {summary}">
			{#each segments as segment (segment.key)}
				{#if segment.value > 0}
					<div class="seg seg--{segment.key}" style="width: {segment.value * 100}%"></div>
				{/if}
			{/each}
		</div>
		<ul class="legend">
			{#each segments as segment (segment.key)}
				<li>
					<span class="dot dot--{segment.key}"></span>
					{segment.label}
					<strong>{pct(segment.value)}%</strong>
				</li>
			{/each}
		</ul>
	</div>
{:else}
	<p class="no-data">Sin datos de sentimiento todavía.</p>
{/if}

<style>
	.sentiment {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	.bar {
		display: flex;
		height: 10px;
		border-radius: var(--radius-pill);
		overflow: hidden;
		background: var(--blue-50);
	}

	.seg {
		height: 100%;
		transition: width 0.3s;
	}
	.seg--positive {
		background: #8fa37e;
	}
	.seg--neutral {
		background: #a8b6c8;
	}
	.seg--negative {
		background: #bd7480;
	}

	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1.25rem;
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.legend li {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.775rem;
		color: var(--muted);
	}

	.legend strong {
		color: var(--text);
		font-weight: 600;
	}

	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}
	.dot--positive {
		background: #8fa37e;
	}
	.dot--neutral {
		background: #a8b6c8;
	}
	.dot--negative {
		background: #bd7480;
	}

	.no-data {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
	}
</style>
