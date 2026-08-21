<script lang="ts">
	import type { OptionCount } from '$lib/types';

	// El agregado de las preguntas estructuradas: cuántos eligieron cada opción.
	// Reemplaza al tag cloud, que solo tiene sentido donde hay texto libre.
	let { distribution }: { distribution: OptionCount[] } = $props();

	const total = $derived(distribution.reduce((sum, o) => sum + o.count, 0));

	// Las barras se escalan contra la opción más votada, no contra el total: con
	// muchas opciones repartidas, escalar contra el total deja todas las barras
	// como una línea y no se distingue cuál ganó.
	const max = $derived(Math.max(1, ...distribution.map((o) => o.count)));

	function pct(count: number): number {
		return total > 0 ? Math.round((count / total) * 100) : 0;
	}
</script>

{#if distribution.length > 0}
	<ul class="dist">
		{#each distribution as option (option.value)}
			<li class="row">
				<span class="label" title={option.label}>{option.label}</span>
				<span class="track">
					<span class="fill" style="width: {(option.count / max) * 100}%"></span>
				</span>
				<span class="value">
					<strong>{option.count}</strong>
					<span class="pct">{pct(option.count)}%</span>
				</span>
			</li>
		{/each}
	</ul>
{:else}
	<p class="no-data">Sin respuestas todavía.</p>
{/if}

<style>
	.dist {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.45rem;
	}

	.row {
		display: grid;
		grid-template-columns: minmax(60px, 8rem) 1fr auto;
		align-items: center;
		gap: 0.6rem;
	}

	.label {
		font-size: 0.8rem;
		color: var(--text);
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.track {
		background: var(--blue-50);
		border: 1px solid var(--blue-100);
		border-radius: var(--radius-pill);
		height: 0.6rem;
		overflow: hidden;
	}

	.fill {
		display: block;
		height: 100%;
		background: var(--blue-400);
		border-radius: var(--radius-pill);
		min-width: 2px;
	}

	.value {
		display: inline-flex;
		align-items: baseline;
		gap: 0.35rem;
		font-size: 0.78rem;
		white-space: nowrap;
	}
	.value strong {
		color: var(--text);
		font-weight: 700;
	}
	.pct {
		color: var(--muted);
	}

	.no-data {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
	}
</style>
