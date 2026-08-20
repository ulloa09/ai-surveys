<!-- src/lib/components/conversation/ProgressBar.svelte -->
<script lang="ts">
	let { covered = 0, total = 0 }: { covered?: number; total?: number } = $props();

	const pct = $derived(total > 0 ? Math.round((covered / total) * 100) : 0);
</script>

{#if total > 0}
	<div
		class="progress-wrap"
		role="progressbar"
		aria-valuenow={covered}
		aria-valuemin={0}
		aria-valuemax={total}
		aria-label="Progreso: {covered} de {total} preguntas respondidas"
	>
		<div class="bar">
			<div class="fill" style="width: {pct}%"></div>
		</div>
		<span class="label">{covered}/{total}</span>
	</div>
{/if}

<style>
	.progress-wrap {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.bar {
		width: 80px;
		height: 4px;
		background: rgba(255, 255, 255, 0.2);
		border-radius: 2px;
		overflow: hidden;
	}

	.fill {
		height: 100%;
		background: var(--success);
		border-radius: 2px;
		transition: width 0.4s ease;
	}

	.label {
		font-size: 0.75rem;
		color: rgba(255, 255, 255, 0.75);
		font-weight: 600;
		white-space: nowrap;
	}

	@media (prefers-reduced-motion: reduce) {
		.fill {
			transition: none;
		}
	}
</style>
