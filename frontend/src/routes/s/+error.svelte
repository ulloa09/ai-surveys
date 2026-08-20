<script lang="ts">
	import { page } from '$app/state';

	const status = $derived(page.status);
	const isNotFound = $derived(status === 404);

	const title = $derived(isNotFound ? 'Encuesta no encontrada' : 'Algo salió mal');
	const message = $derived(
		isNotFound
			? 'El enlace que abriste no corresponde a ninguna encuesta disponible. Verifica el código QR o la dirección e inténtalo de nuevo.'
			: (page.error?.message ?? 'No pudimos cargar esta encuesta. Inténtalo más tarde.')
	);
</script>

<svelte:head>
	<title>{title} — AI Surveys</title>
</svelte:head>

<div class="page">
	<p class="page-wordmark"><span class="mark">AI</span> Surveys</p>

	<main class="card">
		<div class="code">{status}</div>
		<h1>{title}</h1>
		<p>{message}</p>
	</main>
	<footer class="brand">AI Surveys</footer>
</div>

<style>
	.page {
		--navy: var(--blue-900);
		--line: var(--border);
		--surface: var(--bg);

		min-height: 100vh;
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1.25rem;
		padding: 1.5rem 1rem 2rem;
		background: var(--surface);
		color: var(--navy);
		font-family: var(--font);
		-webkit-font-smoothing: antialiased;
		text-align: center;
	}

	.page-wordmark {
		font-family: var(--font-display);
		font-size: 1.15rem;
		font-weight: 500;
		color: var(--blue-700);
		margin: 0;
	}

	.page-wordmark .mark {
		font-weight: 700;
		color: var(--blue-900);
	}

	.card {
		width: min(100%, 480px);
		background: #fff;
		border: 1px solid var(--line);
		border-top: 5px solid var(--blue-900);
		border-radius: var(--radius-md);
		padding: 2.5rem 1.75rem;
		box-shadow: var(--shadow-md);
	}

	.code {
		display: inline-block;
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 0.8rem;
		letter-spacing: 0.14em;
		color: var(--blue-600);
		background: var(--blue-50);
		border: 1px solid var(--border);
		border-radius: var(--radius-pill);
		padding: 0.3rem 0.85rem;
		margin-bottom: 1rem;
	}

	h1 {
		margin: 0 0 0.6rem;
		font-family: var(--font-display);
		font-weight: 600;
		font-size: clamp(1.5rem, 6vw, 1.9rem);
		line-height: 1.15;
	}

	p {
		margin: 0;
		color: var(--muted);
		line-height: 1.6;
		font-size: 0.95rem;
	}

	.brand {
		font-size: 0.78rem;
		color: var(--muted);
	}
</style>
