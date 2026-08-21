<script lang="ts">
	import type { PageData } from './$types';
	import { QUESTION_TYPE_LABELS, respondentLabel } from '$lib/types';
	import type { ResponseQuestion, RespondentResponse, RespondentAnswer, Turn } from '$lib/types';
	import EmptyState from '$lib/components/shared/EmptyState.svelte';
	import ChatBubble from '$lib/components/conversation/ChatBubble.svelte';

	let { data }: { data: PageData } = $props();

	const survey = $derived(data.survey);
	const responses = $derived(data.responses);

	let tab = $state<'question' | 'respondent'>('question');

	function formatDate(dateStr: string | null): string {
		if (!dateStr) return '—';
		return new Date(dateStr).toLocaleDateString('es-MX', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	// ── Por pregunta ─────────────────────────────────────────────────────

	function answersFor(questionId: string): { response: RespondentResponse; answer: RespondentAnswer }[] {
		const out: { response: RespondentResponse; answer: RespondentAnswer }[] = [];
		for (const response of responses.responses) {
			const answer = response.answers.find((a) => a.question_id === questionId);
			if (answer) out.push({ response, answer });
		}
		return out;
	}

	// stopwords muy básico (es/en) para que la frecuencia de palabras en
	// preguntas abiertas no se llene de conectores sin significado.
	const STOPWORDS = new Set([
		'que', 'de', 'la', 'el', 'en', 'y', 'a', 'los', 'las', 'un', 'una', 'es', 'con', 'por', 'para',
		'no', 'se', 'lo', 'me', 'mi', 'su', 'del', 'al', 'como', 'muy', 'más', 'pero', 'si', 'fue',
		'the', 'a', 'an', 'of', 'to', 'and', 'in', 'is', 'it', 'was', 'for', 'on', 'with', 'that', 'this'
	]);

	function topWords(texts: string[], limit = 8): { word: string; count: number }[] {
		const counts = new Map<string, number>();
		for (const text of texts) {
			const words = text.toLowerCase().match(/[\p{L}]{3,}/gu) ?? [];
			for (const w of words) {
				if (STOPWORDS.has(w)) continue;
				counts.set(w, (counts.get(w) ?? 0) + 1);
			}
		}
		return [...counts.entries()]
			.map(([word, count]) => ({ word, count }))
			.sort((a, b) => b.count - a.count)
			.slice(0, limit);
	}

	function distribution(values: string[]): { value: string; count: number; pct: number }[] {
		const counts = new Map<string, number>();
		for (const v of values) counts.set(v, (counts.get(v) ?? 0) + 1);
		const total = values.length || 1;
		return [...counts.entries()]
			.map(([value, count]) => ({ value, count, pct: Math.round((count / total) * 100) }))
			.sort((a, b) => b.count - a.count);
	}

	type QuestionStats =
		| { kind: 'distribution'; rows: { value: string; count: number; pct: number }[] }
		| { kind: 'scale'; avg: number; min: number; max: number; rows: { value: string; count: number; pct: number }[] }
		| { kind: 'words'; words: { word: string; count: number }[] }
		| { kind: 'none' };

	function statsFor(question: ResponseQuestion, entries: { answer: RespondentAnswer }[]): QuestionStats {
		if (entries.length === 0) return { kind: 'none' };
		switch (question.type) {
			case 'single_choice':
			case 'true_false':
			case 'multi_choice':
				return { kind: 'distribution', rows: distribution(entries.map((e) => e.answer.display_value)) };
			case 'linear_scale': {
				const nums = entries.map((e) => Number(e.answer.raw_value)).filter((n) => !Number.isNaN(n));
				if (nums.length === 0) return { kind: 'none' };
				const avg = nums.reduce((a, b) => a + b, 0) / nums.length;
				return {
					kind: 'scale',
					avg: Math.round(avg * 10) / 10,
					min: Math.min(...nums),
					max: Math.max(...nums),
					rows: distribution(entries.map((e) => e.answer.raw_value))
				};
			}
			case 'open_ended': {
				const words = topWords(entries.map((e) => e.answer.raw_value));
				return words.length > 0 ? { kind: 'words', words } : { kind: 'none' };
			}
			default:
				return { kind: 'none' };
		}
	}

	// ── Por alumno ───────────────────────────────────────────────────────

	let selectedResponseId = $state('');
	let searchTerm = $state('');

	const filteredRespondents = $derived(
		searchTerm.trim() === ''
			? responses.responses
			: responses.responses.filter((r) =>
					respondentLabel(r).toLowerCase().includes(searchTerm.trim().toLowerCase())
				)
	);

	const selectedRespondent = $derived(
		responses.responses.find((r) => r.response_id === selectedResponseId) ?? null
	);

	function questionText(questionId: string): string {
		return responses.questions.find((q) => q.question_id === questionId)?.text ?? questionId;
	}

	function questionType(questionId: string): string {
		const type = responses.questions.find((q) => q.question_id === questionId)?.type;
		return type ? (QUESTION_TYPE_LABELS[type] ?? type) : '';
	}

	// ── Conversación (prompt_only / conversational) ─────────────────────
	// Los turns no vienen en /api/surveys/{id}/responses (esa lista puede tener
	// cientos de respuestas). Se cargan bajo demanda, solo para el alumno
	// seleccionado, vía el proxy /api/responses/{id} → GET /api/responses/{id}
	// del backend (#14), con su mismo control de acceso.
	let turnsLoading = $state(false);
	let turnsError = $state<string | null>(null);
	let loadedTurns = $state<Turn[]>([]);
	let loadedForId = $state('');

	// Mientras carga (o si la carga es de OTRO alumno, justo después de cambiar
	// la selección) selectedTurns se queda vacío — así nunca se ve la
	// conversación de la persona anterior pegada un instante bajo el nombre de
	// la nueva.
	const selectedTurns = $derived(loadedForId === selectedResponseId ? loadedTurns : []);

	$effect(() => {
		const id = selectedResponseId;
		if (!id) {
			loadedTurns = [];
			loadedForId = '';
			return;
		}
		turnsLoading = true;
		turnsError = null;
		fetch(`/api/responses/${id}`)
			.then(async (res) => {
				if (!res.ok) throw new Error('No se pudo cargar la conversación.');
				const detail = await res.json();
				loadedTurns = detail.turns ?? [];
				loadedForId = id;
			})
			.catch((err) => {
				turnsError = err instanceof Error ? err.message : 'No se pudo cargar la conversación.';
				loadedTurns = [];
				loadedForId = id;
			})
			.finally(() => {
				turnsLoading = false;
			});
	});
</script>

<svelte:head><title>Respuestas: {survey.title} — AI Surveys</title></svelte:head>

<div class="page">
	<div class="page-hero">
		<div class="hero-inner">
			<div>
				<a href="/dashboard/surveys/{survey.id}/lifecycle" class="back-link">← {survey.title}</a>
				<h1 class="page-title">Respuestas individuales</h1>
				<p class="page-sub">
					{responses.responses.length}
					{responses.responses.length === 1 ? 'respuesta enviada' : 'respuestas enviadas'}
				</p>
			</div>
		</div>
	</div>

	<div class="page-body">
		{#if responses.responses.length === 0}
			<EmptyState
				title="Todavía no hay respuestas enviadas"
				description="En cuanto alguien complete la encuesta, sus respuestas van a aparecer aquí."
			/>
		{:else}
			<div class="tabs">
				<button class="tab" class:tab--active={tab === 'question'} onclick={() => (tab = 'question')}>
					Por pregunta
				</button>
				<button
					class="tab"
					class:tab--active={tab === 'respondent'}
					onclick={() => (tab = 'respondent')}
				>
					Por alumno
				</button>
			</div>

			{#if tab === 'question'}
				{#if responses.questions.length === 0}
					<EmptyState
						title="Esta encuesta no tiene preguntas fijas"
						description="Es una encuesta en modo prompt libre: la IA conduce toda la conversación a partir de un system prompt, sin preguntas para agrupar aquí. Revisa la pestaña “Por alumno” para leer la conversación completa de cada participante."
					/>
				{:else}
					<div class="questions">
						{#each responses.questions as question, i (question.question_id)}
							{@const entries = answersFor(question.question_id)}
							{@const stats = statsFor(question, entries)}
							<section class="card">
								<header class="q-header">
									<span class="q-num">{i + 1}</span>
									<div class="q-heading">
										<h2 class="q-text">{question.text}</h2>
										<div class="q-meta">
											<span class="type-badge">{QUESTION_TYPE_LABELS[question.type] ?? question.type}</span>
											<span class="answer-count"
												>{entries.length} {entries.length === 1 ? 'respuesta' : 'respuestas'}</span
											>
										</div>
									</div>
								</header>

								{#if stats.kind === 'distribution'}
									<div class="block">
										<h3 class="block-title">Distribución</h3>
										<ul class="dist">
											{#each stats.rows as row (row.value)}
												<li class="dist-row">
													<span class="dist-label">{row.value}</span>
													<div class="dist-bar-track">
														<div class="dist-bar" style="width: {row.pct}%"></div>
													</div>
													<span class="dist-count">{row.count} ({row.pct}%)</span>
												</li>
											{/each}
										</ul>
									</div>
								{:else if stats.kind === 'scale'}
									<div class="block">
										<h3 class="block-title">Estadísticas</h3>
										<div class="scale-summary">
											<span><strong>{stats.avg}</strong> promedio</span>
											<span><strong>{stats.min}</strong> mínimo</span>
											<span><strong>{stats.max}</strong> máximo</span>
										</div>
										<ul class="dist">
											{#each stats.rows as row (row.value)}
												<li class="dist-row">
													<span class="dist-label">{row.value}</span>
													<div class="dist-bar-track">
														<div class="dist-bar" style="width: {row.pct}%"></div>
													</div>
													<span class="dist-count">{row.count} ({row.pct}%)</span>
												</li>
											{/each}
										</ul>
									</div>
								{:else if stats.kind === 'words'}
									<div class="block">
										<h3 class="block-title">Palabras frecuentes</h3>
										<div class="words">
											{#each stats.words as w (w.word)}
												<span class="word-chip">{w.word} <strong>{w.count}</strong></span>
											{/each}
										</div>
									</div>
								{/if}

								<div class="block">
									<h3 class="block-title">Todas las respuestas</h3>
									{#if entries.length === 0}
										<p class="no-data">Nadie contestó esta pregunta todavía.</p>
									{:else}
										<ul class="answer-list">
											{#each entries as { response, answer } (response.response_id)}
												<li class="answer-row">
													<span class="answer-who">{respondentLabel(response)}</span>
													<span class="answer-text">{answer.display_value}</span>
												</li>
											{/each}
										</ul>
									{/if}
								</div>
							</section>
						{/each}
					</div>
				{/if}
			{:else}
				<div class="respondent-view">
					<div class="respondent-picker">
						<input
							type="search"
							class="respondent-search"
							placeholder="Buscar alumno o respuesta anónima…"
							bind:value={searchTerm}
						/>
						<ul class="respondent-list">
							{#each filteredRespondents as r (r.response_id)}
								<li>
									<button
										class="respondent-item"
										class:respondent-item--active={r.response_id === selectedResponseId}
										onclick={() => (selectedResponseId = r.response_id)}
									>
										<span class="respondent-name">{respondentLabel(r)}</span>
										<span class="respondent-meta">{formatDate(r.submitted_at)}</span>
									</button>
								</li>
							{:else}
								<li class="no-data">Sin coincidencias.</li>
							{/each}
						</ul>
					</div>

					<div class="respondent-detail">
						{#if !selectedRespondent}
							<EmptyState
								title="Elige un alumno"
								description="Selecciona una respuesta de la lista para ver todo lo que contestó."
							/>
						{:else}
							<div class="card">
								<header class="respondent-header">
									<div>
										<h2 class="q-text">{respondentLabel(selectedRespondent)}</h2>
										<p class="page-sub">Enviada el {formatDate(selectedRespondent.submitted_at)}</p>
									</div>
								</header>

								{#if selectedRespondent.answers.length > 0}
									<ul class="answer-list answer-list--detail">
										{#each selectedRespondent.answers as answer (answer.question_id)}
											<li class="answer-row answer-row--detail">
												<div class="answer-question">
													<span class="answer-question-text">{questionText(answer.question_id)}</span>
													<span class="type-badge">{questionType(answer.question_id)}</span>
												</div>
												<span class="answer-text">{answer.display_value}</span>
											</li>
										{/each}
									</ul>
								{/if}

								<!-- Conversación: prompt_only no tiene preguntas fijas, así que esto
								     es TODA su respuesta. En conversational complementa la lista de
								     arriba con el diálogo real — incluidas las repreguntas de la IA
								     que no quedan resumidas en display_value. -->
								{#if turnsLoading}
									<p class="no-data">Cargando conversación…</p>
								{:else if turnsError}
									<p class="no-data no-data--error">{turnsError}</p>
								{:else if selectedTurns.length > 0}
									<div class="conversation">
										<h3 class="block-title">Conversación</h3>
										<div class="conversation-thread">
											{#each selectedTurns as turn, i (i)}
												<ChatBubble role={turn.role} content={turn.content} />
											{/each}
										</div>
									</div>
								{:else if selectedRespondent.answers.length === 0}
									<p class="no-data">Esta persona no contestó ninguna pregunta.</p>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			{/if}
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

	.hero-inner {
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
		color: var(--muted);
		margin: 0;
	}
	.page-hero .page-sub {
		color: rgba(255, 255, 255, 0.65);
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

	/* ── Tabs ─────────────────────────────────────────────────────────── */
	.tabs {
		display: flex;
		gap: 0.5rem;
		border-bottom: 1.5px solid var(--blue-100);
	}

	.tab {
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 0.6rem 0.25rem;
		margin-bottom: -1.5px;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--muted);
		cursor: pointer;
		transition:
			color 0.15s,
			border-color 0.15s;
	}
	.tab:hover {
		color: var(--blue-700);
	}
	.tab--active {
		color: var(--blue-700);
		border-bottom-color: var(--blue-600);
	}

	/* ── Question cards (reuses analysis card language) ──────────────────*/
	.questions {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

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
		white-space: nowrap;
	}

	.answer-count {
		font-size: 0.775rem;
		color: var(--muted);
	}

	.block {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}
	.block-title {
		font-size: 0.72rem;
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		margin: 0;
	}

	.no-data {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
		list-style: none;
	}

	/* ── Distribution / scale stats ──────────────────────────────────── */
	.dist {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}

	.dist-row {
		display: grid;
		grid-template-columns: minmax(80px, 1fr) 3fr auto;
		align-items: center;
		gap: 0.6rem;
	}

	.dist-label {
		font-size: 0.82rem;
		color: var(--text);
		overflow-wrap: anywhere;
	}

	.dist-bar-track {
		background: var(--blue-50);
		border-radius: var(--radius-pill);
		height: 8px;
		overflow: hidden;
	}
	.dist-bar {
		background: var(--blue-400);
		height: 100%;
		border-radius: var(--radius-pill);
	}

	.dist-count {
		font-size: 0.75rem;
		color: var(--muted);
		white-space: nowrap;
	}

	.scale-summary {
		display: flex;
		gap: 1.5rem;
		font-size: 0.82rem;
		color: var(--muted);
	}
	.scale-summary strong {
		color: var(--text);
		font-size: 1rem;
	}

	.words {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.word-chip {
		font-size: 0.8rem;
		color: var(--blue-700);
		background: var(--blue-50);
		border: 1px solid var(--blue-200);
		border-radius: var(--radius-pill);
		padding: 0.2rem 0.7rem;
	}
	.word-chip strong {
		color: var(--blue-900);
	}

	/* ── Answer list ──────────────────────────────────────────────────── */
	.answer-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.answer-row {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		padding: 0.6rem 0.85rem;
		background: var(--blue-50);
		border: 1px solid var(--blue-100);
		border-radius: var(--radius-md);
	}

	.answer-who {
		font-size: 0.7rem;
		font-weight: 700;
		color: var(--blue-700);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.answer-text {
		font-size: 0.88rem;
		color: var(--text);
		line-height: 1.5;
		overflow-wrap: anywhere;
	}

	/* ── Respondent view ──────────────────────────────────────────────── */
	.respondent-view {
		display: grid;
		grid-template-columns: 280px 1fr;
		gap: 1.5rem;
		align-items: start;
	}

	.respondent-picker {
		background: var(--white);
		border: 1.5px solid var(--blue-100);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-sm);
		padding: 1rem;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		max-height: 70vh;
		overflow: hidden;
	}

	.respondent-search {
		border: 1.5px solid var(--blue-200);
		border-radius: var(--radius-md);
		padding: 0.5rem 0.75rem;
		font-size: 0.85rem;
		color: var(--text);
		background: var(--white);
	}
	.respondent-search:focus {
		outline: none;
		border-color: var(--blue-500);
	}

	.respondent-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		overflow-y: auto;
	}

	.respondent-item {
		width: 100%;
		text-align: left;
		background: none;
		border: 1px solid transparent;
		border-radius: var(--radius-md);
		padding: 0.5rem 0.6rem;
		cursor: pointer;
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		transition:
			background 0.15s,
			border-color 0.15s;
	}
	.respondent-item:hover {
		background: var(--blue-50);
	}
	.respondent-item--active {
		background: var(--blue-50);
		border-color: var(--blue-300);
	}

	.respondent-name {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--text);
		overflow-wrap: anywhere;
	}
	.respondent-meta {
		font-size: 0.72rem;
		color: var(--muted);
	}

	.respondent-detail {
		min-width: 0;
	}
	.respondent-header {
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}
	.respondent-header > div:first-child {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		min-width: 0;
	}

	.answer-list--detail {
		margin-top: 0.25rem;
	}
	.answer-row--detail {
		gap: 0.35rem;
	}
	.answer-question {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
	}
	.answer-question-text {
		font-size: 0.8rem;
		font-weight: 700;
		color: var(--blue-800);
	}

	.no-data--error {
		color: var(--danger);
	}

	/* ── Conversación (prompt_only / conversational) ─────────────────────── */
	.conversation {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-top: 0.5rem;
	}

	.conversation-thread {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		background: var(--blue-50);
		border: 1px solid var(--blue-100);
		border-radius: var(--radius-md);
		padding: 1rem;
		max-height: 32rem;
		overflow-y: auto;
	}

	@media (max-width: 800px) {
		.page-hero {
			padding: 1.75rem 1.5rem;
		}
		.page-body {
			padding: 1.25rem 1.5rem;
		}
		.respondent-view {
			grid-template-columns: 1fr;
		}
		.dist-row {
			grid-template-columns: 1fr;
		}
	}
</style>
