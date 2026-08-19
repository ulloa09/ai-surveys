<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>AI Surveys</title>
</svelte:head>

<div class="status-card">
	<h1>AI Surveys</h1>
	<p>
		Backend: <strong class:ok={data.status === 'ok'} class:down={data.status !== 'ok'}>
			{data.status}
		</strong>
	</p>
	<p>
		Database: <strong class:ok={data.database === 'connected'} class:down={data.database !== 'connected'}>
			{data.database}
		</strong>
	</p>
</div>

<style>
	.status-card {
		max-width: 480px;
		margin: 4rem auto;
		padding: 2rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--white);
	}

	h1 {
		margin: 0 0 1rem;
	}

	.ok {
		color: var(--success, #2e7d32);
	}

	.down {
		color: var(--danger, #c62828);
	}
</style>
