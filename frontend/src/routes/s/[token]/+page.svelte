<script lang="ts">
	import { enhance } from '$app/forms';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);
	const isClosed = $derived(survey.status === 'closed');

	let selectedLanguage = $state(survey.default_language ?? 'es');
	let starting = $state(false);

	// Reanudación: en el montaje buscamos un resume token en localStorage y lo
	// validamos contra el backend. Si sigue vigente, ofrecemos continuar la
	// sesión anterior en lugar de crear una nueva (criterio de aceptación #09).
	let resumeResponseId = $state<string | null>(null);

	// Reanudación del lado del servidor (issue #14): para usuarios autenticados
	// el backend detecta la respuesta en progreso por identidad de sesión, de modo
	// que se puede continuar desde cualquier dispositivo. Tiene prioridad sobre el
	// resume token de localStorage (que solo aplica al mismo dispositivo, #09).
	const serverResume = $derived(data.resumeResponse as { id: string; status: string } | null);

	// Diálogo de confirmación para "Comenzar de nuevo": descartar la sesión previa.
	let showRestartConfirm = $state(false);

	const languageLabels: Record<string, string> = {
		es: 'Español',
		en: 'English'
	};

	// Declaración de anonimato en lenguaje llano según el nivel configurado.
	const anonymity = $derived(
		(
			{
				full: {
					badge: 'Anónima',
					text: 'Tus respuestas son completamente anónimas. No guardamos ningún dato que permita identificarte.'
				},
				partial: {
					badge: 'Pseudónima',
					text: 'Tus respuestas no se vinculan a tu identidad. Solo reconocemos tu dispositivo para evitar envíos duplicados.'
				},
				none: {
					badge: 'Identificada',
					text: 'Tus respuestas quedan vinculadas a tu cuenta institucional.'
				}
			} as const
		)[survey.anonymity_level as 'full' | 'partial' | 'none']
	);

	const resumeKey = $derived(`resume_${survey.public_token}`);

	onMount(async () => {
		if (isClosed) return;
		const token = localStorage.getItem(resumeKey);
		if (!token) return;
		try {
			const res = await fetch(`/s/${survey.public_token}/resume?token=${encodeURIComponent(token)}`);
			const body = await res.json();
			if (body.found) resumeResponseId = body.responseId;
			else localStorage.removeItem(resumeKey);
		} catch {
			// Sin conexión o error: simplemente no ofrecemos reanudar.
		}
	});

	function goToConversation(responseId: string) {
		goto(`/s/${survey.public_token}/conversation?response=${responseId}`);
	}

	function handleContinue() {
		if (resumeResponseId) goToConversation(resumeResponseId);
	}

	// Enhance del formulario de inicio: al crear la respuesta guardamos el resume
	// token y navegamos a la conversación desde el cliente.
	const submitStart = () => {
		starting = true;
		return async ({ result, update }: { result: any; update: () => Promise<void> }) => {
			if (result.type === 'success' && result.data) {
				const { responseId, resumeToken } = result.data;
				if (resumeToken) localStorage.setItem(resumeKey, resumeToken);
				goToConversation(responseId);
				return;
			}
			starting = false;
			await update();
		};
	};
</script>

<svelte:head>
	<title>{survey.title} — AI Surveys</title>
</svelte:head>

<div class="page">
	<p class="page-wordmark"><span class="mark">AI</span> Surveys</p>

	<main class="card">
		<header class="hero">
			<span class="eyebrow"><span class="dot" aria-hidden="true"></span>Encuesta</span>
			<h1>{survey.title}</h1>
			<div class="meta">
				{#if survey.time_estimate_minutes}
					<span class="pill">
						<svg viewBox="0 0 24 24" aria-hidden="true"
							><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg
						>
						≈ {survey.time_estimate_minutes} min
					</span>
				{/if}
				<span class="pill">
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6z" /></svg
					>
					{anonymity.badge}
				</span>
			</div>
		</header>

		<div class="body">
			{#if survey.description}
				<p class="description">{survey.description}</p>
			{/if}

			<p class="anonymity">{anonymity.text}</p>

			{#if isClosed}
				<section class="closed" role="status">
					<h2>Esta encuesta ya cerró</h2>
					<p>«{survey.title}» ya no recibe respuestas. Gracias por tu interés.</p>
				</section>
			{:else}
				{#if serverResume}
					<section class="resume" aria-live="polite">
						<div class="resume-copy">
							<strong>Tienes una sesión sin terminar</strong>
							<span>Puedes continuar justo donde la dejaste, en este o cualquier dispositivo.</span>
						</div>
						<button type="button" class="btn-ghost" onclick={() => goToConversation(serverResume.id)}>
							Continuar donde lo dejaste
						</button>
					</section>
				{:else if resumeResponseId}
					<section class="resume" aria-live="polite">
						<div class="resume-copy">
							<strong>Tienes una sesión sin terminar</strong>
							<span>Puedes continuar justo donde la dejaste.</span>
						</div>
						<button type="button" class="btn-ghost" onclick={handleContinue}>
							Continuar donde lo dejaste
						</button>
					</section>
				{/if}

				<form
					method="POST"
					action={serverResume ? '?/restart' : '?/start'}
					use:enhance={submitStart}
					class="start-form"
				>
					{#if survey.available_languages.length > 1}
						<label class="field">
							<span class="label">Idioma</span>
							<select name="language" bind:value={selectedLanguage}>
								{#each survey.available_languages as language}
									<option value={language}>{languageLabels[language] ?? language}</option>
								{/each}
							</select>
						</label>
					{:else}
						<input type="hidden" name="language" value={survey.default_language} />
					{/if}

					{#if survey.optional_registration}
						<fieldset class="registration">
							<legend>Registro opcional</legend>
							<p class="hint">
								Si compartes tu nombre o correo, tus respuestas dejarán de ser anónimas.
								Ambos campos son opcionales.
							</p>
							<label class="field">
								<span class="label">Nombre</span>
								<input
									type="text"
									name="registered_name"
									autocomplete="name"
									placeholder="Tu nombre (opcional)"
								/>
							</label>
							<label class="field">
								<span class="label">Correo electrónico</span>
								<input
									type="email"
									name="registered_email"
									autocomplete="email"
									placeholder="tucorreo@example.com (opcional)"
								/>
							</label>
						</fieldset>
					{/if}

					{#if form?.error}
						<p class="error" role="alert">{form.error}</p>
					{/if}

					{#if serverResume}
						<!-- previous_id viaja a la acción `restart` para abandonar la sesión previa -->
						<input type="hidden" name="previous_id" value={serverResume.id} />
						<button
							type="button"
							class="btn-secondary"
							disabled={starting}
							onclick={() => (showRestartConfirm = true)}
						>
							Comenzar de nuevo
						</button>
					{:else}
						<button type="submit" class="btn-primary" disabled={starting}>
							{starting ? 'Iniciando…' : 'Comenzar'}
							<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
						</button>
					{/if}

					{#if showRestartConfirm}
						<div class="confirm" role="alertdialog" aria-label="Confirmar empezar de nuevo">
							<p class="confirm-copy">
								Si empiezas de nuevo, tu sesión anterior se descartará y no podrás recuperarla.
								¿Quieres continuar?
							</p>
							<div class="confirm-actions">
								<button
									type="button"
									class="btn-ghost"
									disabled={starting}
									onclick={() => (showRestartConfirm = false)}
								>
									Cancelar
								</button>
								<button type="submit" class="btn-danger" disabled={starting}>
									{starting ? 'Iniciando…' : 'Sí, empezar de nuevo'}
								</button>
							</div>
						</div>
					{/if}
				</form>
			{/if}
		</div>
	</main>

	<footer class="brand">AI Surveys</footer>
</div>

<style>
	.page {
		--navy: var(--blue-900);
		--blue: var(--blue-900);
		--blue-deep: var(--blue-700);
		--sky: var(--blue-400);
		--sky-soft: var(--blue-50);
		--ink: var(--text);
		--muted: #5b6167;
		--line: var(--border);
		--surface: var(--bg);
		--white: #ffffff;

		min-height: 100vh;
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1.25rem;
		padding: 1.5rem 1rem 2rem;
		background: var(--surface);
		color: var(--ink);
		font-family: var(--font);
		-webkit-font-smoothing: antialiased;
	}

	.page-wordmark {
		font-family: var(--font-display);
		font-size: 1.15rem;
		font-weight: 500;
		color: var(--blue-deep);
		margin: 0;
	}

	.page-wordmark .mark {
		font-weight: 700;
		color: var(--navy);
	}

	.card {
		width: min(100%, 560px);
		background: var(--white);
		border-radius: var(--radius-md);
		overflow: hidden;
		box-shadow:
			0 1px 2px rgba(11, 44, 92, 0.06),
			0 18px 48px rgba(11, 44, 92, 0.14);
		border: 1px solid var(--line);
	}

	/* ── Hero band ──────────────────────────────────────────── */
	.hero {
		padding: 1.75rem 1.75rem 1.5rem;
		background: var(--navy);
		color: var(--white);
	}

	.eyebrow {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: rgba(255, 255, 255, 0.85);
	}

	.dot {
		width: 0.55rem;
		height: 0.55rem;
		border-radius: 50%;
		background: var(--white);
		box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.22);
	}

	.hero h1 {
		margin: 0.75rem 0 0;
		font-family: var(--font-display);
		font-weight: 600;
		font-size: clamp(1.65rem, 6vw, 2.1rem);
		line-height: 1.12;
		letter-spacing: -0.01em;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 1.1rem;
	}

	.pill {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.35rem 0.7rem;
		border-radius: 100px;
		background: rgba(255, 255, 255, 0.16);
		border: 1px solid rgba(255, 255, 255, 0.25);
		font-size: 0.8rem;
		font-weight: 600;
		color: var(--white);
	}

	.pill svg {
		width: 0.95rem;
		height: 0.95rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	/* ── Body ───────────────────────────────────────────────── */
	.body {
		padding: 1.5rem 1.75rem 1.75rem;
		display: flex;
		flex-direction: column;
		gap: 1.1rem;
	}

	.description {
		margin: 0;
		color: var(--muted);
		line-height: 1.65;
		font-size: 0.98rem;
	}

	.anonymity {
		margin: 0;
		padding: 0.85rem 1rem;
		background: var(--sky-soft);
		border: 1px solid var(--line);
		border-radius: var(--radius-sm);
		color: var(--blue-deep);
		font-size: 0.9rem;
		line-height: 1.55;
	}

	/* ── Resume banner ──────────────────────────────────────── */
	.resume {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 1rem 1.1rem;
		background: var(--sky-soft);
		border: 1px solid var(--sky);
		border-radius: var(--radius-sm);
	}

	.resume-copy {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}

	.resume-copy strong {
		color: var(--navy);
		font-size: 0.95rem;
	}

	.resume-copy span {
		color: var(--muted);
		font-size: 0.85rem;
	}

	/* ── Form ───────────────────────────────────────────────── */
	.start-form {
		display: flex;
		flex-direction: column;
		gap: 1.1rem;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.label {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--ink);
	}

	select,
	input {
		width: 100%;
		border: 1px solid var(--line);
		border-radius: 10px;
		padding: 0.7rem 0.8rem;
		font: inherit;
		font-size: 0.95rem;
		background: var(--white);
		color: var(--ink);
		transition:
			border-color 0.15s ease,
			box-shadow 0.15s ease;
	}

	select:focus,
	input:focus {
		outline: none;
		border-color: var(--blue);
		box-shadow: 0 0 0 3px rgba(30, 91, 176, 0.16);
	}

	.registration {
		border: 1px dashed var(--line);
		border-radius: var(--radius-sm);
		padding: 1rem 1.1rem 1.1rem;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.8rem;
	}

	.registration legend {
		padding: 0 0.4rem;
		font-size: 0.82rem;
		font-weight: 700;
		color: var(--blue-deep);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.hint {
		margin: 0;
		font-size: 0.82rem;
		line-height: 1.5;
		color: var(--muted);
	}

	/* ── Buttons ────────────────────────────────────────────── */
	.btn-primary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		width: 100%;
		border: 0;
		border-radius: var(--radius-sm);
		padding: 0.9rem 1.2rem;
		background: var(--navy);
		color: var(--white);
		font-size: 1rem;
		font-weight: 700;
		cursor: pointer;
		box-shadow: var(--shadow-sm);
		transition:
			transform 0.12s ease,
			box-shadow 0.12s ease,
			opacity 0.12s ease;
	}

	.btn-primary:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 12px 26px rgba(30, 91, 176, 0.34);
	}

	.btn-primary:active:not(:disabled) {
		transform: translateY(0);
	}

	.btn-primary:disabled {
		opacity: 0.65;
		cursor: progress;
	}

	.btn-primary svg {
		width: 1.1rem;
		height: 1.1rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 2.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.btn-ghost {
		align-self: flex-start;
		border: 1px solid var(--blue);
		border-radius: 10px;
		padding: 0.6rem 1rem;
		background: var(--white);
		color: var(--blue-deep);
		font-size: 0.9rem;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.12s ease;
	}

	.btn-ghost:hover {
		background: var(--sky-soft);
	}

	.btn-ghost:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Botón "Comenzar de nuevo": secundario, menos prominente que el primario. */
	.btn-secondary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		width: 100%;
		border: 1px solid var(--line);
		border-radius: var(--radius-sm);
		padding: 0.85rem 1.2rem;
		background: var(--white);
		color: var(--blue-deep);
		font-size: 0.98rem;
		font-weight: 700;
		cursor: pointer;
		transition:
			border-color 0.12s ease,
			background 0.12s ease;
	}

	.btn-secondary:hover:not(:disabled) {
		border-color: var(--sky);
		background: var(--sky-soft);
	}

	.btn-secondary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* ── Confirmación de "Comenzar de nuevo" ────────────────── */
	.confirm {
		display: flex;
		flex-direction: column;
		gap: 0.9rem;
		padding: 1rem 1.1rem;
		background: #fff7ed;
		border: 1px solid #fed7aa;
		border-radius: var(--radius-sm);
	}

	.confirm-copy {
		margin: 0;
		color: #9a3412;
		font-size: 0.9rem;
		line-height: 1.55;
	}

	.confirm-actions {
		display: flex;
		gap: 0.6rem;
	}

	.confirm-actions .btn-ghost {
		flex: 1;
	}

	.btn-danger {
		flex: 1;
		border: 0;
		border-radius: 10px;
		padding: 0.6rem 1rem;
		background: #c2410c;
		color: var(--white);
		font-size: 0.9rem;
		font-weight: 700;
		cursor: pointer;
		transition:
			background 0.12s ease,
			opacity 0.12s ease;
	}

	.btn-danger:hover:not(:disabled) {
		background: #9a3412;
	}

	.btn-danger:disabled {
		opacity: 0.65;
		cursor: progress;
	}

	/* ── States ─────────────────────────────────────────────── */
	.closed {
		padding: 1.25rem 1.25rem;
		background: #fff7ed;
		border: 1px solid #fed7aa;
		border-radius: var(--radius-sm);
	}

	.closed h2 {
		margin: 0 0 0.4rem;
		font-family: var(--font-display);
		font-size: 1.2rem;
		color: #9a3412;
	}

	.closed p {
		margin: 0;
		color: #9a3412;
		font-size: 0.9rem;
		line-height: 1.55;
	}

	.error {
		margin: 0;
		padding: 0.75rem 0.9rem;
		background: #fef2f2;
		border: 1px solid #fecaca;
		border-radius: 10px;
		color: #991b1b;
		font-size: 0.88rem;
	}

	.brand {
		font-size: 0.78rem;
		color: var(--muted);
		text-align: center;
		letter-spacing: 0.02em;
	}

	@media (min-width: 480px) {
		.resume {
			flex-direction: row;
			align-items: center;
			justify-content: space-between;
		}

		.btn-ghost {
			align-self: auto;
			flex-shrink: 0;
		}
	}
</style>
