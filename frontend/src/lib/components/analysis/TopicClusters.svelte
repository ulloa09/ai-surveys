<script lang="ts">
	import type { TopicCluster } from '$lib/types';

	let { clusters }: { clusters: TopicCluster[] } = $props();

	// El tema más mencionado marca la escala: el resto se dibuja proporcional a
	// él, para que la nube se lea de un vistazo sin importar el volumen absoluto.
	const max = $derived(Math.max(1, ...clusters.map((c) => c.count)));

	// Tres pesos discretos en vez de un tamaño continuo — más legible y evita
	// que un tema con 1 mención quede ilegible frente a otro con 200.
	function weight(count: number): 'lg' | 'md' | 'sm' {
		const ratio = count / max;
		if (ratio >= 0.66) return 'lg';
		if (ratio >= 0.33) return 'md';
		return 'sm';
	}
</script>

{#if clusters.length > 0}
	<ul class="cloud">
		{#each clusters as cluster (cluster.tag)}
			<li class="tag tag--{weight(cluster.count)}">
				{cluster.tag}
				<span class="count">{cluster.count}</span>
			</li>
		{/each}
	</ul>
{:else}
	<p class="no-data">Sin temas detectados todavía.</p>
{/if}

<style>
	.cloud {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.tag {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		border-radius: var(--radius-pill);
		padding: 0.2rem 0.7rem;
		font-weight: 600;
		color: var(--blue-700);
		background: var(--blue-50);
		border: 1px solid var(--blue-200);
	}

	.tag--lg {
		font-size: 0.9rem;
		background: var(--blue-100);
		border-color: var(--blue-300);
	}
	.tag--md {
		font-size: 0.825rem;
	}
	.tag--sm {
		font-size: 0.75rem;
		color: var(--muted);
	}

	.count {
		background: var(--white);
		border-radius: var(--radius-pill);
		padding: 0 0.4rem;
		font-size: 0.7rem;
		font-weight: 700;
		color: var(--blue-600);
	}

	.no-data {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
	}
</style>
