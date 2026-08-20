<script lang="ts">
	import { enhance } from '$app/forms';
	import { SURVEY_MODES, TERMINATION_MODES } from '$lib/types';
	import type { ActionData, PageData } from './$types';

	let { data, form }: { data: PageData; form: ActionData } = $props();

	const survey = $derived(data.survey);

	let mode = $state(survey.mode);
	let systemPrompt = $state(survey.system_prompt ?? '');
	let terminationMode = $state(survey.termination_mode);
	let turnLimit = $state(survey.turn_limit ?? 12);
	let timeEstimateMinutes = $state(survey.time_estimate_minutes ?? 15);
	let spanishEnabled = $state(survey.available_languages?.includes('es') ?? true);
	let englishEnabled = $state(survey.available_languages?.includes('en') ?? false);
	let defaultLanguage = $state<'es' | 'en'>(survey.default_language ?? 'es');

	$effect(() => {
		mode = survey.mode;
		systemPrompt = survey.system_prompt ?? '';
		terminationMode = survey.termination_mode;
		turnLimit = survey.turn_limit ?? 12;
		timeEstimateMinutes = survey.time_estimate_minutes ?? 15;
		spanishEnabled = survey.available_languages?.includes('es') ?? true;
		englishEnabled = survey.available_languages?.includes('en') ?? false;
		defaultLanguage = survey.default_language ?? 'es';
	});

	// Al menos un idioma debe seguir marcado, y el idioma default debe
	// pertenecer a los idiomas disponibles — el backend rechaza lo contrario.
	$effect(() => {
		if (!spanishEnabled && !englishEnabled) spanishEnabled = true;
		if (defaultLanguage === 'es' && !spanishEnabled) defaultLanguage = englishEnabled ? 'en' : 'es';
		if (defaultLanguage === 'en' && !englishEnabled) defaultLanguage = spanishEnabled ? 'es' : 'en';
	});

	const promptRequired = $derived(mode !== 'form');
</script>

<svelte:head>
	<title>Configurar: {survey.title} — AI Surveys</title>
</svelte:head>

<p class="breadcrumb"><a href="/dashboard/surveys">← Encuestas</a></p>

<div class="header">
	<h1>{survey.title}</h1>
	<span class="badge">{survey.status}</span>
</div>

<nav class="subnav">
	<a href="/dashboard/surveys/{survey.id}" class="active">Modo &amp; IA</a>
	<a href="/dashboard/surveys/{survey.id}/questions">Preguntas</a>
	<a href="/dashboard/surveys/{survey.id}/lifecycle">Ciclo de vida</a>
	<a href="/admin/surveys/{survey.id}">Configuración básica</a>
</nav>

{#if form?.error}
	<p class="banner error" role="alert">{form.error}</p>
{:else if form?.updated}
	<p class="banner ok">Configuración guardada.</p>
{/if}

<form class="card" method="POST" action="?/updateConfig" use:enhance>
	<h2>Modo de conversación</h2>
	<div class="mode-cards">
		{#each SURVEY_MODES as m (m.value)}
			<label class="mode-card" class:selected={mode === m.value}>
				<input type="radio" name="mode" value={m.value} bind:group={mode} />
				<span class="mode-title">{m.label}</span>
				<span class="mode-desc">{m.description}</span>
				<span class="mode-example">{m.example}</span>
			</label>
		{/each}
	</div>

	{#if mode === 'form'}
		<p class="hint">
			En este modo la IA sigue tus preguntas de la pestaña "Preguntas" sin un system prompt —
			esa sección queda oculta abajo.
		</p>
	{/if}

	<label>
		System prompt {promptRequired ? '(obligatorio)' : '(opcional)'}
		<textarea
			name="system_prompt"
			rows="4"
			required={promptRequired}
			bind:value={systemPrompt}
			placeholder="Eres un entrevistador que explora la experiencia del semestre..."
		></textarea>
	</label>

	<h2>Terminación de la conversación</h2>
	<div class="termination-options">
		{#each TERMINATION_MODES as t (t.value)}
			<label class="term-option" class:selected={terminationMode === t.value}>
				<input type="radio" name="termination_mode" value={t.value} bind:group={terminationMode} />
				<span class="term-title">{t.label}</span>
				<span class="term-desc">{t.description}</span>
			</label>
		{/each}
	</div>

	{#if terminationMode === 'turn_limit' || terminationMode === 'combination'}
		<label class="inline">
			Límite de turnos
			<input type="number" name="turn_limit" min="1" bind:value={turnLimit} />
		</label>
	{/if}

	{#if terminationMode === 'time_estimate' || terminationMode === 'combination'}
		<label class="inline">
			Duración estimada (minutos)
			<input type="number" name="time_estimate_minutes" min="1" bind:value={timeEstimateMinutes} />
		</label>
	{/if}

	{#if terminationMode === 'combination'}
		<p class="hint">Los tres criterios están activos a la vez; el primero en cumplirse termina la conversación.</p>
	{/if}

	<h2>Idiomas</h2>
	<fieldset class="language-panel">
		<legend>Idiomas disponibles</legend>
		<label class="check">
			<input type="checkbox" name="available_languages" value="es" bind:checked={spanishEnabled} />
			Español (es)
		</label>
		<label class="check">
			<input type="checkbox" name="available_languages" value="en" bind:checked={englishEnabled} />
			Inglés (en)
		</label>
		<label>
			Idioma por defecto
			<select name="default_language" bind:value={defaultLanguage}>
				<option value="es" disabled={!spanishEnabled}>Español</option>
				<option value="en" disabled={!englishEnabled}>Inglés</option>
			</select>
		</label>
		<p class="hint-inline">
			El respondiente elige entre los idiomas marcados; si solo hay uno, no ve selector. La IA
			responde en el idioma elegido.
		</p>
	</fieldset>

	<button class="primary" type="submit">Guardar configuración</button>
</form>

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
	}

	h2 {
		font-size: 1rem;
		color: var(--blue-900);
		margin: 0 0 0.75rem;
	}

	.mode-cards {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		margin-bottom: 1.25rem;
	}

	.mode-card {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.75rem 0.9rem;
		cursor: pointer;
	}
	.mode-card.selected {
		border-color: var(--blue-400);
		background: var(--blue-50);
	}
	.mode-card input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}
	.mode-title {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 0.9rem;
		color: var(--blue-900);
	}
	.mode-desc {
		font-size: 0.8rem;
		color: var(--text);
	}
	.mode-example {
		font-size: 0.76rem;
		color: var(--muted);
		font-style: italic;
	}

	.hint {
		background: var(--warning-lt);
		border-left: 3px solid var(--warning);
		color: var(--warning);
		font-size: 0.82rem;
		padding: 0.5rem 0.75rem;
		border-radius: var(--radius-sm);
		margin: 0 0 1rem;
	}

	.language-panel {
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.9rem 1rem;
		margin: 0 0 1.25rem;
	}
	.language-panel legend {
		padding: 0 0.25rem;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--blue-900);
	}
	.language-panel label.check {
		font-weight: 400;
	}
	.hint-inline {
		font-size: 0.78rem;
		color: var(--muted);
		margin: 0.5rem 0 0;
	}

	label {
		display: block;
		margin-bottom: 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--blue-900);
	}
	label.inline {
		max-width: 260px;
	}
	label.check {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	textarea,
	input[type='number'] {
		display: block;
		width: 100%;
		margin-top: 0.35rem;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 0.9rem;
		box-sizing: border-box;
		font-family: inherit;
	}

	.termination-options {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}

	.term-option {
		display: flex;
		flex-direction: column;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.6rem 0.85rem;
		cursor: pointer;
		font-weight: 400;
	}
	.term-option.selected {
		border-color: var(--blue-400);
		background: var(--blue-50);
	}
	.term-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}
	.term-title {
		font-family: var(--font-display);
		font-weight: 600;
		font-size: 0.85rem;
		color: var(--blue-900);
	}
	.term-desc {
		font-size: 0.78rem;
		color: var(--muted);
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
</style>
