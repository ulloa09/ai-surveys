<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import type { PageData } from './$types';
	import type { Question } from '$lib/types';
	import ChatBubble from '$lib/components/conversation/ChatBubble.svelte';
	import ProgressBar from '$lib/components/conversation/ProgressBar.svelte';

	let { data }: { data: PageData } = $props();

	const survey = $derived(data.survey);
	const responseId = $derived(data.responseId);
	const token = $derived(survey.public_token as string);
	const mode = $derived(survey.mode as 'conversational' | 'form' | 'prompt_only');

	interface Message {
		id: string;
		role: 'user' | 'assistant' | 'system';
		content: string;
		streaming?: boolean;
		questionId?: string; // pregunta asociada (para allow_revisit)
		questionType?: string; // tipo de pregunta asociada
	}

	let questions = $state<Question[]>([]);
	// true una vez que loadQuestions() resolvió (con o sin preguntas). Sirve
	// para distinguir "todavía cargando" de "cargó y no hay preguntas" — un
	// modo que no es prompt_only necesita preguntas para poder mostrar algo;
	// si el creador activó la encuesta sin agregar ninguna, sin esta bandera
	// la pantalla se queda en blanco para siempre, sin ninguna pista de qué
	// pasó (issue reportado: encuesta abre pero no muestra nada).
	let questionsLoaded = $state(false);
	let answeredQs = $state(new Set<string>());

	let waitingForFollowup = $state(false);
	let aiThinking = $state(false);
	let submitted = $state(false);
	let submitting = $state(false);
	let showConfirm = $state(false);

	let sessionEnded = $state(false); // el engine marcó pending_submission
	let closingRequested = $state(false); // el cierre de la IA ya se pidió (una sola vez)
	let blocked = $state(false); // el engine cerró la sesión (respuestas inválidas repetidas)
	let submittedIncomplete = $state(false); // se envió con preguntas sin responder

	let textValue = $state('');
	let selectedRadio = $state('');
	let multiSelected = $state(new Set<string>());
	let scaleValue = $state<number | null>(null);
	let matrixAnswers = $state<Record<string, string>>({});
	let rankingOrder = $state<string[]>([]);
	let dragFrom = $state<number | null>(null);
	let followupText = $state('');
	let reconnecting = $state(false); // pérdida de red durante el streaming

	// Revisit: edición de respuestas previas
	// Cada mensaje de usuario puede tener un questionId asociado para saber qué editar
	interface EditableMessage extends Message {
		questionId?: string; // ID de la pregunta que responde este mensaje
		questionType?: string; // tipo de pregunta
	}
	let messages = $state<EditableMessage[]>([]);
	let editingQuestionId = $state<string | null>(null); // pregunta en modo edición
	let editValue = $state('');
	let editRadio = $state('');
	let editMulti = $state(new Set<string>());
	let editScale = $state<number | null>(null);
	let editMatrix = $state<Record<string, string>>({});
	let editRanking = $state<string[]>([]);
	let editDragFrom = $state<number | null>(null);

	let chatEl = $state<HTMLElement | null>(null);

	// Timer para time_estimate — cuenta regresiva desde que carga la página.
	// Cuando llega a cero, muestra un banner y activa el botón de envío.
	let timeExpired = $state(false);
	let timeLeft = $state<number | null>(null);
	let timerInterval: ReturnType<typeof setInterval> | null = null;

	const termMode = $derived(survey.termination_mode as string);
	const allowRevisit = $derived(survey.allow_revisit as boolean);
	const timeEstimateSec = $derived(
		(survey.time_estimate_minutes as number | null)
			? (survey.time_estimate_minutes as number) * 60
			: null
	);

	interface ScaleOpts {
		min: number;
		max: number;
		min_label?: string;
		max_label?: string;
	}
	interface Choice {
		value: string;
		label: string;
	}
	interface ChoiceOpts {
		choices: Choice[];
	}
	interface RankOpts {
		items: { value: string; label: string }[];
	}
	interface MatrixOpts {
		rows: string[];
		columns: string[];
	}

	const requiredQs = $derived(questions.filter((q) => q.required));
	const requiredTotal = $derived(requiredQs.length);
	const requiredDone = $derived(requiredQs.filter((q) => answeredQs.has(q.id)).length);
	// Todas las requeridas cubiertas (true también cuando no hay requeridas).
	const allRequiredDone = $derived(requiredDone >= requiredTotal);
	// El usuario ya aportó algo — evita ofrecer submit de una sesión vacía.
	const hasParticipated = $derived(
		mode === 'prompt_only' ? messages.some((m) => m.role === 'user') : answeredQs.size > 0
	);

	const activeQ = $derived<Question | null>(
		mode === 'prompt_only' ? null : (questions.find((q) => !answeredQs.has(q.id)) ?? null)
	);

	// Salvaguarda: un modo que no es prompt_only necesita al menos una
	// pregunta para poder mostrar algo — si el creador activó la encuesta sin
	// agregar ninguna, esto evita una pantalla en blanco sin explicación.
	const noQuestionsConfigured = $derived(
		mode !== 'prompt_only' && questionsLoaded && questions.length === 0
	);

	// El submit siempre es una acción explícita del usuario. Exige SIEMPRE alguna
	// participación: una sesión sin nada aportado no debe poder enviarse aunque el
	// tiempo/turnos la hayan terminado — si no, quedaría una respuesta 'submitted'
	// vacía que se ve como incompleta en el visor (bug de "empezar de nuevo").
	// Con participación, se envía cuando la sesión terminó o cuando las requeridas
	// están cubiertas.
	const canSubmit = $derived(
		!submitted && !blocked && hasParticipated && (sessionEnded || allRequiredDone)
	);

	onMount(async () => {
		if (!responseId) {
			goto(`/s/${token}`);
			return;
		}
		if (mode !== 'prompt_only') {
			await Promise.all([loadQuestions(), syncAnswered()]);
		}

		// Reconstruir el historial de turnos al reanudar una sesión previa.
		await loadTurns();
		await checkStatus();

		// Si se reanuda una sesión ya terminada, el cierre de la IA ya se generó y
		// persistió antes (loadTurns lo reconstruye): no lo regeneres.
		if (sessionEnded) closingRequested = true;

		// Modo B: la IA arranca la conversación con un mensaje de bienvenida,
		// solo si la sesión es nueva (sin historial previo).
		if (mode === 'prompt_only' && messages.length === 0 && !sessionEnded && !blocked) {
			await askAI('[WELCOME]');
		}

		// Iniciar countdown si el modo incluye time_estimate
		if (timeEstimateSec && (termMode === 'time_estimate' || termMode === 'combination')) {
			timeLeft = timeEstimateSec;
			timerInterval = setInterval(() => {
				timeLeft = (timeLeft ?? 0) - 1;
				if ((timeLeft ?? 0) <= 0) {
					timeLeft = 0;
					timeExpired = true;
					if (timerInterval) clearInterval(timerInterval);
					notifyTimeExpired();
				}
			}, 1000);
		}
	});

	// Avisa al backend que el tiempo terminó. El backend solo marca
	// pending_submission si las requeridas están cubiertas; si faltan,
	// la sesión sigue viva y el banner explica qué falta. Nunca se hace
	// submit automático — el usuario decide con el botón.
	async function notifyTimeExpired() {
		try {
			const res = await fetch(`/s/${token}/expire`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ responseId })
			});
			if (res.ok) {
				const { status } = await res.json().catch(() => ({ status: '' }));
				if (status === 'pending_submission') {
					sessionEnded = true;
					await showClosing();
				}
			}
		} catch {}
	}

	// Limpiar el timer cuando el componente se destruye
	onDestroy(() => {
		if (timerInterval) clearInterval(timerInterval);
	});

	// Carga el historial para reconstruir el chat al reanudar una sesión.
	// El backend persiste cada pregunta mostrada (turno assistant) y cada
	// respuesta del usuario (turno user), así que el transcript es la fuente
	// de verdad. Para allow_revisit, el questionId de cada respuesta se infiere
	// del turno assistant anterior cuando coincide con el texto de una pregunta.
	async function loadTurns() {
		try {
			const res = await fetch(`/s/${token}/turn?responseId=${responseId}`);
			if (!res.ok) return;
			const data = await res.json().catch(() => ({}));
			const turns: { role: string; content: string }[] = data.turns ?? [];

			// Datos previos a la estabilización pueden traer señales embebidas.
			const prefixes = ['[FOLLOWUP_NEEDED]', '[FOLLOWUP_DONE]', '[FOLLOWUP_CLOSE]', '[WELCOME]'];
			const cleanContent = (text: string) => {
				for (const p of prefixes) {
					if (text.startsWith(p)) return text.slice(p.length).trim();
				}
				return text;
			};

			const rebuilt: Message[] = [];
			for (let i = 0; i < turns.length; i++) {
				const content = cleanContent(turns[i].content);
				if (!content) continue;
				if (turns[i].role === 'assistant') {
					rebuilt.push({ id: uid(), role: 'assistant', content, streaming: false });
					continue;
				}
				const prev = turns[i - 1];
				const q =
					prev?.role === 'assistant'
						? questions.find((q) => q.text === prev.content)
						: undefined;
				rebuilt.push({
					id: uid(),
					role: 'user',
					content,
					streaming: false,
					questionId: q?.id,
					questionType: q?.type
				});
			}

			if (rebuilt.length) {
				messages = rebuilt;
				await scrollBottom();
			}
		} catch {
			// Sin conexión o error al reconstruir: continuar con el chat vacío.
		}
	}

	async function loadQuestions() {
		try {
			const res = await fetch(`/s/${token}/questions`);
			if (res.ok) questions = await res.json();
		} catch {
		} finally {
			questionsLoaded = true;
		}
	}

	async function syncAnswered() {
		try {
			const res = await fetch(`/s/${token}/answers?responseId=${responseId}`);
			if (res.ok) {
				const { answered } = await res.json();
				answeredQs = new Set(answered ?? []);
			}
		} catch {}
	}

	// Sincroniza el estado de la sesión con el backend. Nunca envía nada
	// automáticamente: cuando la sesión termina, el usuario ve la última
	// respuesta de la IA y decide con el botón «Terminar encuesta».
	async function checkStatus() {
		try {
			const res = await fetch(`/s/${token}/response-status?responseId=${responseId}`);
			if (res.ok) {
				const { status } = await res.json();
				if (status === 'pending_submission') {
					sessionEnded = true;
				} else if (status === 'abandoned') {
					blocked = true;
				} else if (status === 'submitted') {
					// Con allow_revisit, submitted no bloquea — el usuario puede editar.
					// Sin allow_revisit, marcar sesión como terminada.
					if (!allowRevisit) {
						sessionEnded = true;
					}
				}
			}
		} catch {}
	}

	// Editar una respuesta previa (solo si allow_revisit está activo)
	async function editAnswer(questionId: string, questionType: string) {
		if (!allowRevisit || aiThinking) return;
		editingQuestionId = questionId;
		// Reset de todos los controles de edición
		editValue = '';
		editRadio = '';
		editMulti = new Set();
		editScale = null;
		editMatrix = {};
		const q = questions.find((q) => q.id === questionId);
		editRanking =
			q?.type === 'ranking' ? ((q.options as RankOpts)?.items?.map((i) => i.value) ?? []) : [];
	}

	function canSubmitEdit(): boolean {
		const q = questions.find((q) => q.id === editingQuestionId);
		if (!q) return false;
		switch (q.type) {
			case 'open_ended':
				return editValue.trim().length > 0;
			case 'single_choice':
				return editRadio !== '';
			case 'multi_choice':
				return editMulti.size > 0;
			case 'true_false':
				return editRadio !== '';
			case 'linear_scale':
				return editScale !== null;
			case 'ranking':
				return editRanking.length > 0;
			case 'matrix':
				return !!(q.options as MatrixOpts)?.rows?.every((r) => editMatrix[r]);
			default:
				return false;
		}
	}

	function buildEditValue(): string {
		const q = questions.find((q) => q.id === editingQuestionId);
		if (!q) return '';
		switch (q.type) {
			case 'open_ended':
				return editValue.trim();
			case 'single_choice':
				return editRadio;
			case 'true_false':
				return editRadio;
			case 'linear_scale':
				return String(editScale);
			case 'multi_choice':
				return JSON.stringify([...editMulti]);
			case 'ranking':
				return JSON.stringify(editRanking.map((v, i) => ({ position: i + 1, value: v })));
			case 'matrix':
				return JSON.stringify(editMatrix);
			default:
				return '';
		}
	}

	function buildEditLabel(): string {
		const q = questions.find((q) => q.id === editingQuestionId);
		if (!q) return '';
		const opts = q.options as any;
		switch (q.type) {
			case 'open_ended':
				return editValue.trim();
			case 'single_choice':
				return opts?.choices?.find((c: Choice) => c.value === editRadio)?.label ?? editRadio;
			case 'multi_choice':
				return [...editMulti]
					.map((v) => opts?.choices?.find((c: Choice) => c.value === v)?.label ?? v)
					.join(', ');
			case 'true_false':
				return editRadio === 'true' ? 'Sí' : 'No';
			case 'linear_scale':
				return `${editScale} de ${(opts as ScaleOpts)?.min ?? 1} a ${(opts as ScaleOpts)?.max ?? 10}`;
			case 'ranking':
				return editRanking.map((v) => opts?.items?.find((i: any) => i.value === v)?.label ?? v).join(' → ');
			case 'matrix':
				return Object.entries(editMatrix)
					.map(([r, c]) => `${r}: ${c}`)
					.join(', ');
			default:
				return '';
		}
	}

	async function submitEdit() {
		if (!editingQuestionId || !canSubmitEdit()) return;
		const qid = editingQuestionId;
		const val = buildEditValue();
		const label = buildEditLabel();
		editingQuestionId = null;

		// Actualizar el mensaje en el chat con el label legible
		messages = messages.map((m) => (m.questionId === qid ? { ...m, content: label } : m));

		// Guardar en backend — el backend reactiva la respuesta a in_progress
		const q = questions.find((q) => q.id === qid);
		const endpoint = q?.type === 'open_ended' ? 'open-answer' : 'answers';
		const res = await fetch(`/s/${token}/${endpoint}`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ responseId, questionId: qid, value: val })
		});

		if (res.ok) {
			sessionEnded = false;
			closingRequested = false; // si la sesión vuelve a terminar, permite un cierre nuevo
		}
	}

	function onEditDragStart(i: number) {
		editDragFrom = i;
	}
	function onEditDragOver(e: DragEvent, i: number) {
		e.preventDefault();
		if (editDragFrom === null || editDragFrom === i) return;
		const arr = [...editRanking];
		const [moved] = arr.splice(editDragFrom, 1);
		arr.splice(i, 0, moved);
		editRanking = arr;
		editDragFrom = i;
	}
	function onEditDragEnd() {
		editDragFrom = null;
	}

	function uid() {
		return Math.random().toString(36).slice(2);
	}

	function addMsg(role: Message['role'], content: string, opts: Partial<Message> = {}): Message {
		const msg: Message = { id: uid(), role, content, ...opts };
		messages = [...messages, msg];
		scrollBottom();
		return msg;
	}

	function updateMsg(id: string, content: string, opts: Partial<Message> = {}) {
		messages = messages.map((m) => (m.id === id ? { ...m, content, ...opts } : m));
	}

	async function scrollBottom() {
		await tick();
		chatEl?.scrollTo({ top: chatEl.scrollHeight });
	}

	function resetInputs() {
		textValue = '';
		selectedRadio = '';
		multiSelected = new Set();
		scaleValue = null;
		matrixAnswers = {};
		rankingOrder =
			activeQ?.type === 'ranking'
				? ((activeQ.options as RankOpts)?.items?.map((i) => i.value) ?? [])
				: [];
	}

	$effect(() => {
		if (activeQ) resetInputs();
	});

	// StreamResult describe el desenlace de un turno en streaming.
	//   ok       — el stream se completó correctamente.
	//   ended    — el engine cerró la sesión (HTTP 409).
	//   netFail  — se agotaron los reintentos por pérdida de red.
	//   aiFail   — el proveedor de IA falló (evento SSE de error).
	//   fullText — texto acumulado de la IA.
	//   status   — estado final de la respuesta reportado por el engine
	//              (evento SSE `status` emitido al terminar el stream).
	interface StreamResult {
		ok: boolean;
		ended: boolean;
		netFail: boolean;
		aiFail: boolean;
		fullText: string;
		status: string;
	}

	const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));

	// streamTurn hace el POST a /turn y lee el stream SSE, reintentando de forma
	// automática ante pérdida de red (fallo al conectar o corte a mitad del stream)
	// con backoff exponencial. Mientras reintenta muestra el banner de reconexión.
	// El backend no persiste turnos cuando el cliente se desconecta a mitad del
	// stream (guard por ctx cancelado), así que reintentar el POST es idempotente.
	async function streamTurn(
		signal: string,
		onToken: (text: string) => void
	): Promise<StreamResult> {
		const maxAttempts = 3;
		const fail = (over: Partial<StreamResult>): StreamResult => ({
			ok: false,
			ended: false,
			netFail: false,
			aiFail: false,
			fullText: '',
			status: '',
			...over
		});
		for (let attempt = 0; attempt < maxAttempts; attempt++) {
			let res: Response;
			try {
				res = await fetch(`/s/${token}/turn`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ responseId, message: signal })
				});
			} catch {
				// Fallo de red al establecer la conexión.
				if (attempt < maxAttempts - 1) {
					reconnecting = true;
					await delay(1000 * 2 ** attempt);
					continue;
				}
				reconnecting = false;
				return fail({ netFail: true });
			}

			// El servidor respondió: 409 = sesión cerrada; otros errores no se reintentan.
			if (res.status === 409) {
				reconnecting = false;
				return fail({ ended: true });
			}
			if (!res.ok || !res.body) {
				reconnecting = false;
				return fail({ aiFail: res.status === 503 });
			}

			try {
				const reader = res.body.getReader();
				const decoder = new TextDecoder();
				let buffer = '',
					fullText = '',
					status = '',
					eventName = '',
					aiFail = false;
				while (true) {
					const { done, value } = await reader.read();
					if (done) break;
					buffer += decoder.decode(value, { stream: true });
					const lines = buffer.split('\n');
					buffer = lines.pop() ?? '';
					for (const line of lines) {
						if (line.startsWith('event:')) {
							eventName = line.slice(6).trim();
							continue;
						}
						if (line.startsWith('data: ')) {
							const chunk = line.slice(6);
							if (eventName === 'status') {
								status = chunk.trim();
								eventName = '';
								continue;
							}
							if (eventName === 'error') {
								aiFail = true;
								eventName = '';
								continue;
							}
							eventName = '';
							if (chunk === '[DONE]' || chunk === '[ERROR]') continue;
							fullText += chunk;
							onToken(fullText);
						}
					}
				}
				reconnecting = false;
				if (aiFail) return fail({ aiFail: true });
				return { ok: true, ended: false, netFail: false, aiFail: false, fullText, status };
			} catch {
				// Corte de red a mitad del stream: reintentar desde cero. El texto ya
				// mostrado se reemplaza por el del nuevo intento vía onToken.
				if (attempt < maxAttempts - 1) {
					reconnecting = true;
					await delay(1000 * 2 ** attempt);
					continue;
				}
				reconnecting = false;
				return fail({ netFail: true });
			}
		}
		reconnecting = false;
		return fail({ netFail: true });
	}

	// Pide a la IA el cierre de la entrevista cuando la sesión terminó: agradece,
	// reconoce lo compartido e indica cómo finalizar («Terminar encuesta»).
	// Se dispara una sola vez por carga de página (closingRequested) para no
	// duplicar cierres ni gastar tokens de más. [SESSION_CLOSE] está permitido en
	// pending_submission y el backend lo persiste como turno assistant.
	async function showClosing() {
		if (closingRequested) return;
		closingRequested = true;

		const aiMsg = addMsg('assistant', '', { streaming: true });
		const { ok, fullText } = await streamTurn('[SESSION_CLOSE]', (text) => {
			updateMsg(aiMsg.id, text, { streaming: true });
			scrollBottom();
		});

		if (!ok || !fullText) {
			messages = messages.filter((m) => m.id !== aiMsg.id);
			return;
		}

		// Salvaguarda: eliminar cualquier oración que termine en pregunta.
		const sentences = fullText.split(/(?<=[.!])\s+/);
		const closingSentences = sentences.filter((s) => !s.trim().endsWith('?'));
		const cleaned =
			closingSentences.length > 0
				? closingSentences.join(' ')
				: fullText.replace(/\s*[^.!?]*\?/g, '').trim();
		updateMsg(aiMsg.id, cleaned || fullText, { streaming: false });
	}

	// Resultado de un turno de IA para quien lo invoca:
	//   ended — la sesión quedó en pending_submission (mostrar botón Terminar).
	//   empty — la IA no produjo texto (turno ahorrado por el engine).
	interface AskResult {
		ended: boolean;
		empty: boolean;
	}

	// Llama a la IA con una señal y streamea la respuesta en una burbuja.
	// El estado final viene en el propio stream (evento `status`), así que la
	// sesión nunca se cierra sin que el usuario vea la última respuesta.
	async function askAI(signal: string): Promise<AskResult> {
		aiThinking = true;
		const aiMsg = addMsg('assistant', '', { streaming: true });

		try {
			const r = await streamTurn(signal, (text) => {
				updateMsg(aiMsg.id, text, { streaming: true });
				scrollBottom();
			});

			// El engine cerró la sesión antes de este turno (409).
			if (r.ended) {
				messages = messages.filter((m) => m.id !== aiMsg.id);
				await checkStatus();
				return { ended: true, empty: true };
			}

			if (!r.ok) {
				updateMsg(
					aiMsg.id,
					r.netFail
						? 'No se pudo reconectar. Revisa tu conexión e inténtalo de nuevo.'
						: 'El asistente no está disponible en este momento. Puedes continuar con la encuesta.',
					{ streaming: false }
				);
				return { ended: false, empty: false };
			}

			if (r.status === 'abandoned') blocked = true;
			const ended = r.status === 'pending_submission';

			if (r.fullText === '') {
				// Turno ahorrado por el engine (límite alcanzado o sin followup).
				messages = messages.filter((m) => m.id !== aiMsg.id);
				if (ended) {
					sessionEnded = true;
					await showClosing();
				}
				return { ended, empty: true };
			}

			updateMsg(aiMsg.id, r.fullText, { streaming: false });
			if (ended) {
				sessionEnded = true;
				await showClosing();
			}
			return { ended, empty: false };
		} finally {
			aiThinking = false;
		}
	}

	async function submitAnswer(value: string, humanLabel: string) {
		if (!activeQ || aiThinking) return;
		const q = activeQ;

		// Pregunta y respuesta quedan en el historial — el mismo par que el
		// backend persiste en el transcript.
		const qBubble = addMsg('assistant', q.text);
		const uBubble = addMsg('user', humanLabel, { questionId: q.id, questionType: q.type });

		const endpoint = q.type === 'open_ended' ? 'open-answer' : 'answers';
		let res: Response | null = null;
		try {
			res = await fetch(`/s/${token}/${endpoint}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ responseId, questionId: q.id, value })
			});
		} catch {}

		if (!res || !res.ok) {
			const body = res ? await res.json().catch(() => ({})) : {};
			messages = messages.filter((m) => m.id !== uBubble.id && m.id !== qBubble.id);
			if (body.error === 'low_quality_answer') {
				// Followup de recuperación: se pide reformular UNA vez, sin gastar tokens.
				addMsg(
					'assistant',
					'¿Podrías contarme un poco más? Tu respuesta parece muy breve o poco clara — inténtalo con tus propias palabras.'
				);
			} else if (body.error === 'response_blocked') {
				blocked = true;
			} else if (res && res.status === 422) {
				addMsg('system', 'Tu respuesta no pudo validarse. Revisa lo que seleccionaste e inténtalo de nuevo.');
			} else {
				addMsg('system', 'No se pudo guardar tu respuesta. Revisa tu conexión e inténtalo de nuevo.');
			}
			return; // la pregunta sigue activa — la card se muestra de nuevo
		}

		await syncAnswered();
		await checkStatus();
		if (blocked) {
			resetInputs();
			return;
		}

		// Si esta respuesta cerró la sesión (cobertura completa), muestra el cierre
		// de la IA en lugar de seguir con followups.
		if (sessionEnded) {
			await showClosing();
			resetInputs();
			return;
		}

		// Modo híbrido (issue #06, Mode C): cuando la pregunta tiene ai_followup, la
		// IA profundiza para extraer más información — sea abierta o estructurada. La
		// repregunta se refiere SIEMPRE a lo que el participante acaba de responder
		// (el porqué de su elección, un ejemplo o el impacto) y nunca reproduce una
		// pregunta ya planteada (el system prompt lo prohíbe explícitamente). El
		// participante responde en el followup-row. Si el admin no quiere profundizar
		// en una pregunta, deja ai_followup en false y la conversación avanza directo.
		//
		// El modo 'form' (Mode A, "preguntas fijas") nunca profundiza: son las
		// preguntas del admin y nada más. El backend lo hace cumplir igual (devuelve
		// un turno vacío), pero cortamos acá para no gastar la llamada a la IA.
		if (mode !== 'form' && q.ai_followup) {
			const { ended, empty } = await askAI('[FOLLOWUP_NEEDED]');
			if (!ended && !empty) {
				waitingForFollowup = true;
				return;
			}
		}
		resetInputs();
	}

	async function submitFollowup() {
		if (!followupText.trim()) return;
		const text = followupText.trim();
		followupText = '';
		waitingForFollowup = false;

		addMsg('user', text);

		// El agradecimiento de la IA se muestra directamente desde el stream
		// del FOLLOWUP_DONE — sin llamadas extra al proveedor.
		await askAI(`[FOLLOWUP_DONE] ${text}`);
		await syncAnswered();
		resetInputs();
	}

	function handleSubmit() {
		if (!canSubmit || aiThinking) return;
		const optionalLeft = questions.filter((q) => !q.required && !answeredQs.has(q.id));
		if (optionalLeft.length > 0 && !sessionEnded && !timeExpired) {
			showConfirm = true;
			return;
		}
		doSubmit();
	}

	async function doSubmit() {
		showConfirm = false;
		submitting = true;
		try {
			const res = await fetch(`/s/${token}/submit`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ responseId })
			});
			if (res.ok) {
				submittedIncomplete = mode !== 'prompt_only' && questions.some((q) => !answeredQs.has(q.id));
				submitted = true;
			} else if (res.status === 422) {
				addMsg('system', 'Aún faltan preguntas obligatorias por responder. Completa las preguntas pendientes para poder terminar.');
				await syncAnswered();
			} else if (res.status === 409) {
				const body = await res.json().catch(() => ({}));
				if (body.error === 'response already submitted') {
					submitted = true;
				} else {
					addMsg('system', 'No se pudieron enviar tus respuestas en este momento. Inténtalo de nuevo.');
				}
			} else {
				addMsg('system', 'No se pudieron enviar tus respuestas. Inténtalo de nuevo en un momento.');
			}
		} catch {
			addMsg('system', 'Sin conexión. Revisa tu red e inténtalo de nuevo.');
		} finally {
			submitting = false;
		}
	}

	function canSubmitForm(): boolean {
		if (!activeQ) return false;
		switch (activeQ.type) {
			case 'open_ended':
				return textValue.trim().length > 0;
			case 'single_choice':
				return selectedRadio !== '';
			case 'multi_choice':
				return multiSelected.size > 0;
			case 'true_false':
				return selectedRadio !== '';
			case 'linear_scale':
				return scaleValue !== null;
			case 'ranking':
				return rankingOrder.length > 0;
			case 'matrix':
				return !!(activeQ.options as MatrixOpts)?.rows?.every((r) => matrixAnswers[r]);
			default:
				return false;
		}
	}

	function buildValue(): string {
		if (!activeQ) return '';
		switch (activeQ.type) {
			case 'open_ended':
				return textValue.trim();
			case 'single_choice':
				return selectedRadio;
			case 'true_false':
				return selectedRadio;
			case 'linear_scale':
				return String(scaleValue);
			// Array JSON de values seleccionados
			case 'multi_choice':
				return JSON.stringify([...multiSelected]);
			// Array JSON con posición explícita para facilitar visualización
			case 'ranking':
				return JSON.stringify(rankingOrder.map((v, i) => ({ position: i + 1, value: v })));
			// Objeto JSON row_value → column_value
			case 'matrix':
				return JSON.stringify(matrixAnswers);
			default:
				return '';
		}
	}

	function buildLabel(): string {
		if (!activeQ) return '';
		const opts = activeQ.options as any;
		switch (activeQ.type) {
			case 'open_ended':
				return textValue.trim();
			case 'single_choice':
				return opts?.choices?.find((c: Choice) => c.value === selectedRadio)?.label ?? selectedRadio;
			case 'multi_choice':
				return [...multiSelected]
					.map((v) => opts?.choices?.find((c: Choice) => c.value === v)?.label ?? v)
					.join(', ');
			case 'true_false':
				return selectedRadio === 'true' ? 'Sí' : 'No';
			case 'linear_scale':
				return `${scaleValue} de ${(opts as ScaleOpts)?.min ?? 1} a ${(opts as ScaleOpts)?.max ?? 10}`;
			case 'ranking':
				return rankingOrder.map((v) => opts?.items?.find((i: any) => i.value === v)?.label ?? v).join(' → ');
			case 'matrix':
				return Object.entries(matrixAnswers)
					.map(([r, c]) => `${r}: ${c}`)
					.join(', ');
			default:
				return '';
		}
	}

	function onDragStart(i: number) {
		dragFrom = i;
	}
	function onDragOver(e: DragEvent, i: number) {
		e.preventDefault();
		if (dragFrom === null || dragFrom === i) return;
		const arr = [...rankingOrder];
		const [moved] = arr.splice(dragFrom, 1);
		arr.splice(i, 0, moved);
		rankingOrder = arr;
		dragFrom = i;
	}
	function onDragEnd() {
		dragFrom = null;
	}
</script>

<svelte:head>
	<title>{survey.title} — AI Surveys</title>
</svelte:head>

{#if submitted}
	<div class="ty-page">
		<p class="page-wordmark"><span class="mark">AI</span> Surveys</p>
		<div class="ty-card">
			<div class="ty-icon">✓</div>
			{#if submittedIncomplete}
				<h1>¡Gracias por tu tiempo!</h1>
				<p>Las respuestas que compartiste en <strong>{survey.title}</strong> han sido registradas.</p>
				<p class="ty-sub">Tu participación es valiosa aunque no hayas podido responder todas las preguntas.</p>
			{:else}
				<h1>¡Gracias por participar!</h1>
				<p>Tus respuestas para <strong>{survey.title}</strong> han sido registradas.</p>
				<p class="ty-sub">Tu opinión ayuda a mejorar la experiencia.</p>
			{/if}
		</div>
		<footer class="brand">AI Surveys</footer>
	</div>
{:else if blocked}
	<div class="ty-page">
		<p class="page-wordmark"><span class="mark">AI</span> Surveys</p>
		<div class="ty-card">
			<div class="ty-icon ty-icon-neutral">•</div>
			<h1>La sesión ha finalizado</h1>
			<p>No pudimos registrar respuestas válidas para <strong>{survey.title}</strong>, así que la sesión se cerró.</p>
			<p class="ty-sub">Si quieres participar, contacta a la persona que organizó la encuesta.</p>
		</div>
		<footer class="brand">AI Surveys</footer>
	</div>
{:else}
	<div class="shell">
		<header class="header">
			<div class="hinner">
				<span class="dot-live"></span>
				<span class="survey-title">{survey.title}</span>
				{#if timeLeft !== null && !timeExpired}
					<span class="time-chip" class:time-warn={(timeLeft ?? 0) <= 60}>
						⏱ {Math.floor((timeLeft ?? 0) / 60)}:{String((timeLeft ?? 0) % 60).padStart(2, '0')}
					</span>
				{/if}
				<ProgressBar covered={requiredDone} total={requiredTotal} />
			</div>
		</header>

		<main class="main" bind:this={chatEl}>
			<div class="content">
				{#each messages as msg (msg.id)}
					<ChatBubble
						role={msg.role}
						content={msg.content}
						streaming={msg.streaming}
						onEdit={allowRevisit && msg.role === 'user' && msg.questionId && !activeQ && !aiThinking && !sessionEnded && !timeExpired
							? () => editAnswer(msg.questionId!, msg.questionType ?? '')
							: null}
					/>
					<!-- Edit card inline — aparece justo debajo del mensaje que se edita -->
					{#if editingQuestionId && msg.questionId === editingQuestionId}
						{@const editQ = questions.find((q) => q.id === editingQuestionId)}
						<div class="edit-card">
							<p class="edit-label">Editando: <strong>{editQ?.text ?? ''}</strong></p>

							{#if editQ?.type === 'open_ended'}
								<textarea
									class="textarea"
									bind:value={editValue}
									placeholder="Nueva respuesta…"
									rows="3"
									autofocus
									onkeydown={(e) => {
										if (e.key === 'Enter' && e.ctrlKey) {
											e.preventDefault();
											if (canSubmitEdit()) submitEdit();
										}
									}}
								></textarea>
								<p class="hint-text">Ctrl+Enter para confirmar</p>
							{:else if editQ?.type === 'single_choice'}
								<div class="choices">
									{#each (editQ.options as ChoiceOpts)?.choices ?? [] as opt (opt.value)}
										<label class="choice" class:sel={editRadio === opt.value}>
											<input type="radio" name="edit_sc" value={opt.value} bind:group={editRadio} />
											<span>{opt.label}</span>
										</label>
									{/each}
								</div>
							{:else if editQ?.type === 'multi_choice'}
								<div class="choices">
									{#each (editQ.options as ChoiceOpts)?.choices ?? [] as opt (opt.value)}
										<label class="choice" class:sel={editMulti.has(opt.value)}>
											<input
												type="checkbox"
												value={opt.value}
												checked={editMulti.has(opt.value)}
												onchange={(e) => {
													const s = new Set(editMulti);
													e.currentTarget.checked ? s.add(opt.value) : s.delete(opt.value);
													editMulti = s;
												}}
											/>
											<span>{opt.label}</span>
										</label>
									{/each}
								</div>
							{:else if editQ?.type === 'true_false'}
								<div class="tf-row">
									<label class="choice tf" class:sel={editRadio === 'true'}>
										<input type="radio" name="edit_tf" value="true" bind:group={editRadio} />
										<span>✓ Sí</span>
									</label>
									<label class="choice tf" class:sel={editRadio === 'false'}>
										<input type="radio" name="edit_tf" value="false" bind:group={editRadio} />
										<span>✗ No</span>
									</label>
								</div>
							{:else if editQ?.type === 'linear_scale'}
								{@const opts = editQ.options as ScaleOpts}
								<div class="scale-wrap">
									<div class="scale-ends">
										<span>{opts?.min_label ?? opts?.min}</span>
										<span>{opts?.max_label ?? opts?.max}</span>
									</div>
									<input type="range" class="slider" min={opts?.min} max={opts?.max} step="1" bind:value={editScale} />
									<div class="scale-val">{editScale ?? '—'}</div>
								</div>
							{:else if editQ?.type === 'ranking'}
								{@const opts = editQ.options as RankOpts}
								<div class="rank-list">
									{#each editRanking as val, i (val)}
										{@const item = opts?.items?.find((it) => it.value === val)}
										<div
											class="rank-item"
											class:dragging={editDragFrom === i}
											draggable="true"
											ondragstart={() => onEditDragStart(i)}
											ondragover={(e) => onEditDragOver(e, i)}
											ondragend={onEditDragEnd}
										>
											<span class="rnum">{i + 1}</span>
											<span class="dh">⠿</span>
											<span>{item?.label ?? val}</span>
										</div>
									{/each}
								</div>
							{:else if editQ?.type === 'matrix'}
								{@const opts = editQ.options as MatrixOpts}
								<div class="matrix-scroll">
									<div class="matrix" style="--cols:{opts?.columns?.length ?? 4}">
										<div class="mrow mhead">
											<span></span>
											{#each opts?.columns ?? [] as col}<span class="mcol-lbl">{col}</span>{/each}
										</div>
										{#each opts?.rows ?? [] as row}
											<div class="mrow">
												<span class="mrow-lbl">{row}</span>
												{#each opts?.columns ?? [] as col}
													<label class="mcell">
														<input
															type="radio"
															name="edit_m_{row}"
															value={col}
															checked={editMatrix[row] === col}
															onchange={() => {
																editMatrix = { ...editMatrix, [row]: col };
															}}
														/>
													</label>
												{/each}
											</div>
										{/each}
									</div>
								</div>
							{/if}

							<div class="edit-footer">
								<button class="btn-cancel-edit" onclick={() => (editingQuestionId = null)}>Cancelar</button>
								<button class="btn-confirm" onclick={submitEdit} disabled={!canSubmitEdit()}>Guardar →</button>
							</div>
						</div>
					{/if}
				{/each}

				{#if aiThinking && !reconnecting}
					<div class="typing"><span></span><span></span><span></span></div>
				{/if}

				{#if reconnecting}
					<div class="reconnect-msg" role="status" aria-live="polite">
						<span class="reconnect-spinner" aria-hidden="true"></span>
						Conexión perdida, reintentando…
					</div>
				{/if}

				{#if timeExpired && !sessionEnded && !allRequiredDone}
					<div class="time-expired-msg">
						⏱ El tiempo terminó, pero aún faltan respuestas obligatorias. Responde las preguntas restantes para poder terminar la encuesta.
					</div>
				{:else if timeExpired && sessionEnded}
					<div class="time-expired-msg">
						⏱ El tiempo de la encuesta ha concluido. Revisa tus respuestas y presiona «Terminar encuesta» cuando estés listo.
					</div>
				{:else if sessionEnded}
					<div class="session-end-msg">
						Has llegado al final de la sesión. Cuando estés listo, presiona «Terminar encuesta» para enviar tus respuestas.
					</div>
				{/if}

				{#if noQuestionsConfigured}
					<div class="no-questions-msg" role="alert">
						Esta encuesta todavía no tiene preguntas configuradas, así que no hay nada que responder por aquí. Avísale a quien la organizó — necesita agregar al menos una pregunta antes de compartir el link.
					</div>
				{/if}

				{#if activeQ && !waitingForFollowup && !aiThinking}
					<div class="question-card">
						<p class="question-text">{activeQ.text}</p>

						{#if activeQ.type === 'open_ended'}
							<textarea
								class="textarea"
								bind:value={textValue}
								placeholder="Escribe tu respuesta…"
								rows="3"
								onkeydown={(e) => {
									if (e.key === 'Enter' && e.ctrlKey) {
										e.preventDefault();
										if (canSubmitForm()) submitAnswer(buildValue(), buildLabel());
									}
								}}
							></textarea>
							<p class="hint-text">Ctrl+Enter para enviar</p>
						{:else if activeQ.type === 'single_choice'}
							<div class="choices">
								{#each (activeQ.options as ChoiceOpts)?.choices ?? [] as opt (opt.value)}
									<label class="choice" class:sel={selectedRadio === opt.value}>
										<input type="radio" name="sc" value={opt.value} bind:group={selectedRadio} />
										<span>{opt.label}</span>
									</label>
								{/each}
							</div>
						{:else if activeQ.type === 'multi_choice'}
							<div class="choices">
								{#each (activeQ.options as ChoiceOpts)?.choices ?? [] as opt (opt.value)}
									<label class="choice" class:sel={multiSelected.has(opt.value)}>
										<input
											type="checkbox"
											value={opt.value}
											checked={multiSelected.has(opt.value)}
											onchange={(e) => {
												const s = new Set(multiSelected);
												e.currentTarget.checked ? s.add(opt.value) : s.delete(opt.value);
												multiSelected = s;
											}}
										/>
										<span>{opt.label}</span>
									</label>
								{/each}
							</div>
						{:else if activeQ.type === 'true_false'}
							<div class="tf-row">
								<label class="choice tf" class:sel={selectedRadio === 'true'}>
									<input type="radio" name="tf" value="true" bind:group={selectedRadio} />
									<span>✓ Sí</span>
								</label>
								<label class="choice tf" class:sel={selectedRadio === 'false'}>
									<input type="radio" name="tf" value="false" bind:group={selectedRadio} />
									<span>✗ No</span>
								</label>
							</div>
						{:else if activeQ.type === 'linear_scale'}
							{@const opts = activeQ.options as ScaleOpts}
							<div class="scale-wrap">
								<div class="scale-ends">
									<span>{opts?.min_label ?? opts?.min}</span>
									<span>{opts?.max_label ?? opts?.max}</span>
								</div>
								<input type="range" class="slider" min={opts?.min} max={opts?.max} step="1" bind:value={scaleValue} />
								<div class="scale-val">{scaleValue ?? '—'}</div>
							</div>
						{:else if activeQ.type === 'ranking'}
							{@const opts = activeQ.options as RankOpts}
							<div class="rank-list">
								{#each rankingOrder as val, i (val)}
									{@const item = opts?.items?.find((it) => it.value === val)}
									<div
										class="rank-item"
										class:dragging={dragFrom === i}
										draggable="true"
										ondragstart={() => onDragStart(i)}
										ondragover={(e) => onDragOver(e, i)}
										ondragend={onDragEnd}
									>
										<span class="rnum">{i + 1}</span>
										<span class="dh">⠿</span>
										<span>{item?.label ?? val}</span>
									</div>
								{/each}
							</div>
						{:else if activeQ.type === 'matrix'}
							{@const opts = activeQ.options as MatrixOpts}
							<div class="matrix-scroll">
								<div class="matrix" style="--cols:{opts?.columns?.length ?? 4}">
									<div class="mrow mhead">
										<span></span>
										{#each opts?.columns ?? [] as col}<span class="mcol-lbl">{col}</span>{/each}
									</div>
									{#each opts?.rows ?? [] as row}
										<div class="mrow">
											<span class="mrow-lbl">{row}</span>
											{#each opts?.columns ?? [] as col}
												<label class="mcell">
													<input
														type="radio"
														name="m_{row}"
														value={col}
														checked={matrixAnswers[row] === col}
														onchange={() => {
															matrixAnswers = { ...matrixAnswers, [row]: col };
														}}
													/>
												</label>
											{/each}
										</div>
									{/each}
								</div>
							</div>
						{/if}

						{#if activeQ.type === 'open_ended'}
							<p class="type-hint">Escribe con tus propias palabras.</p>
						{:else if activeQ.type === 'linear_scale'}
							<p class="type-hint">Mueve el control para elegir un número.</p>
						{:else if activeQ.type === 'single_choice'}
							<p class="type-hint">Elige una sola opción.</p>
						{:else if activeQ.type === 'multi_choice'}
							<p class="type-hint">Puedes elegir una o varias opciones.</p>
						{:else if activeQ.type === 'true_false'}
							<p class="type-hint">Elige Sí o No.</p>
						{:else if activeQ.type === 'ranking'}
							<p class="type-hint">Arrastra para ordenar.</p>
						{:else if activeQ.type === 'matrix'}
							<p class="type-hint">Selecciona una opción por cada fila.</p>
						{/if}

						<button
							class="btn-confirm"
							onclick={() => {
								if (canSubmitForm()) submitAnswer(buildValue(), buildLabel());
							}}
							disabled={!canSubmitForm() || aiThinking}
						>
							Confirmar →
						</button>
					</div>
				{:else if waitingForFollowup && !sessionEnded}
					<div class="followup-row">
						<input
							class="followup-input"
							bind:value={followupText}
							placeholder="Escribe algo más…"
							onkeydown={(e) => {
								if (e.key === 'Enter' && !e.shiftKey && followupText.trim()) {
									e.preventDefault();
									submitFollowup();
								}
							}}
							autofocus
						/>
						<button class="btn-send" onclick={submitFollowup} disabled={!followupText.trim()}>→</button>
					</div>
				{:else if mode === 'prompt_only' && !sessionEnded && !timeExpired && !aiThinking}
					<div class="followup-row">
						<input
							class="followup-input"
							bind:value={followupText}
							placeholder="Escribe tu respuesta…"
							onkeydown={(e) => {
								if (e.key === 'Enter' && !e.shiftKey && followupText.trim()) {
									e.preventDefault();
									const t = followupText.trim();
									followupText = '';
									addMsg('user', t);
									askAI(t);
								}
							}}
						/>
						<button
							class="btn-send"
							onclick={() => {
								const t = followupText.trim();
								if (!t) return;
								followupText = '';
								addMsg('user', t);
								askAI(t);
							}}>→</button
						>
					</div>
				{/if}

				{#if canSubmit}
					<button class="btn-submit" onclick={handleSubmit} disabled={submitting || aiThinking}>
						{submitting ? 'Enviando…' : sessionEnded || timeExpired ? 'Terminar encuesta ✓' : 'Enviar respuestas ✓'}
					</button>
				{/if}
			</div>
		</main>
	</div>

	{#if showConfirm}
		<div class="overlay" role="dialog" aria-modal="true">
			<div class="confirm-card">
				<h2>¿Enviar respuestas?</h2>
				<p>Hay preguntas opcionales sin responder.</p>
				<div class="confirm-actions">
					<button class="btn-ghost" onclick={() => (showConfirm = false)}>Seguir respondiendo</button>
					<button class="btn-primary-sm" onclick={doSubmit}>Enviar de todas formas</button>
				</div>
			</div>
		</div>
	{/if}
{/if}

<style>
	:global(*) {
		box-sizing: border-box;
		margin: 0;
		padding: 0;
	}
	.shell {
		display: flex;
		flex-direction: column;
		height: 100dvh;
		background: var(--bg);
		font-family: var(--font);
		color: var(--text);
	}
	.header {
		background: var(--blue-900);
		padding: 0.75rem 1rem;
		flex-shrink: 0;
	}
	.hinner {
		max-width: 680px;
		margin: 0 auto;
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.dot-live {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: var(--success);
		flex-shrink: 0;
		animation: pulse 2s infinite;
	}
	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.4;
		}
	}
	.survey-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: rgba(255, 255, 255, 0.9);
		flex: 1;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.main {
		flex: 1;
		overflow-y: auto;
		padding: 1.5rem 1rem;
	}
	.content {
		max-width: 680px;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
	.typing {
		display: flex;
		gap: 4px;
		align-items: center;
		padding: 0.75rem 1rem;
		background: white;
		border-radius: var(--radius-md);
		width: fit-content;
		box-shadow: 0 1px 4px rgba(11, 44, 92, 0.08);
	}
	.typing span {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--blue-900);
		animation: bounce 1.2s infinite;
	}
	.typing span:nth-child(2) {
		animation-delay: 0.2s;
	}
	.typing span:nth-child(3) {
		animation-delay: 0.4s;
	}
	@keyframes bounce {
		0%,
		80%,
		100% {
			transform: translateY(0);
		}
		40% {
			transform: translateY(-5px);
		}
	}
	.session-end-msg {
		text-align: center;
		font-size: 0.875rem;
		color: var(--muted);
		padding: 0.75rem 1rem;
		background: var(--blue-50);
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
	}
	.no-questions-msg {
		text-align: center;
		font-size: 0.9rem;
		line-height: 1.55;
		color: var(--warning);
		padding: 1rem 1.25rem;
		background: var(--warning-lt);
		border-radius: var(--radius-sm);
		border: 1px solid var(--warning);
	}
	.reconnect-msg {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		text-align: center;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--warning);
		padding: 0.75rem 1rem;
		background: var(--warning-lt);
		border-radius: var(--radius-sm);
		border: 1px solid var(--warning);
	}
	.reconnect-spinner {
		width: 14px;
		height: 14px;
		border-radius: 50%;
		border: 2px solid var(--warning);
		border-top-color: var(--warning);
		animation: spin 0.8s linear infinite;
		flex-shrink: 0;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.time-expired-msg {
		text-align: center;
		font-size: 0.9rem;
		color: var(--warning);
		padding: 1rem 1.25rem;
		background: var(--warning-lt);
		border-radius: var(--radius-sm);
		border: 1px solid var(--warning);
		font-weight: 500;
	}
	.time-chip {
		font-size: 0.78rem;
		font-weight: 600;
		color: rgba(255, 255, 255, 0.85);
		background: rgba(255, 255, 255, 0.15);
		padding: 0.2rem 0.6rem;
		border-radius: 100px;
		flex-shrink: 0;
	}
	.time-chip.time-warn {
		background: rgba(239, 68, 68, 0.3);
		color: white;
		animation: pulse 1s infinite;
	}
	.question-card {
		background: white;
		border-radius: var(--radius-md);
		padding: 1.5rem;
		box-shadow:
			0 1px 4px rgba(11, 44, 92, 0.08),
			0 4px 16px rgba(11, 44, 92, 0.06);
		border: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
	.question-text {
		font-size: 1.1rem;
		font-weight: 600;
		color: var(--blue-900);
		line-height: 1.4;
	}
	.textarea {
		width: 100%;
		border: 1.5px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.75rem;
		font: inherit;
		font-size: 0.925rem;
		resize: vertical;
		outline: none;
		transition: border-color 0.15s;
	}
	.textarea:focus {
		border-color: var(--blue-900);
	}
	.hint-text {
		font-size: 0.75rem;
		color: var(--muted);
	}
	.type-hint {
		font-size: 0.78rem;
		color: var(--muted);
		background: var(--blue-50);
		border-radius: 8px;
		padding: 0.4rem 0.75rem;
		border-left: 3px solid var(--blue-900);
	}
	.choices {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.choice {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.65rem 0.875rem;
		border: 1.5px solid var(--border);
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: 0.9rem;
		background: white;
		transition:
			border-color 0.15s,
			background 0.15s;
	}
	.choice:hover {
		border-color: var(--blue-900);
		background: var(--blue-50);
	}
	.choice.sel {
		border-color: var(--blue-900);
		background: var(--blue-50);
		font-weight: 500;
	}
	.choice input {
		accent-color: var(--blue-900);
		flex-shrink: 0;
	}
	.tf-row {
		display: flex;
		gap: 0.6rem;
	}
	.tf {
		flex: 1;
		justify-content: center;
		font-weight: 600;
	}
	.scale-wrap {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.scale-ends {
		display: flex;
		justify-content: space-between;
		font-size: 0.8rem;
		color: var(--muted);
	}
	.slider {
		width: 100%;
		accent-color: var(--blue-900);
		cursor: pointer;
	}
	.scale-val {
		text-align: center;
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--blue-900);
		line-height: 1;
	}
	.rank-list {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.rank-item {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.6rem 0.875rem;
		border: 1.5px solid var(--border);
		border-radius: var(--radius-sm);
		background: white;
		cursor: grab;
		font-size: 0.9rem;
		user-select: none;
		transition:
			border-color 0.15s,
			opacity 0.15s;
	}
	.rank-item:hover {
		border-color: var(--blue-900);
	}
	.rank-item.dragging {
		opacity: 0.45;
	}
	.rnum {
		width: 22px;
		height: 22px;
		border-radius: 50%;
		background: var(--blue-900);
		color: white;
		font-size: 0.72rem;
		font-weight: 700;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.dh {
		color: var(--muted);
	}
	.matrix-scroll {
		overflow-x: auto;
	}
	.matrix {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		min-width: 280px;
	}
	.mrow {
		display: grid;
		grid-template-columns: 1fr repeat(var(--cols, 4), 48px);
		gap: 0.2rem;
		align-items: center;
	}
	.mcol-lbl {
		text-align: center;
		font-size: 0.72rem;
		color: var(--muted);
		font-weight: 600;
	}
	.mrow-lbl {
		font-size: 0.875rem;
		color: var(--text);
		padding-right: 0.5rem;
	}
	.mcell {
		display: flex;
		justify-content: center;
		align-items: center;
		height: 40px;
		cursor: pointer;
	}
	.mcell input {
		accent-color: var(--blue-900);
		width: 18px;
		height: 18px;
	}
	.btn-confirm {
		align-self: flex-end;
		background: var(--blue-900);
		color: white;
		border: none;
		border-radius: var(--radius-sm);
		padding: 0.65rem 1.5rem;
		font: inherit;
		font-size: 0.9rem;
		font-weight: 600;
		cursor: pointer;
		transition:
			background 0.15s,
			transform 0.1s;
	}
	.btn-confirm:hover:not(:disabled) {
		background: var(--blue-700);
		transform: translateY(-1px);
	}
	.btn-confirm:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
	.followup-row {
		display: flex;
		gap: 0.5rem;
		background: white;
		border-radius: 24px;
		padding: 0.25rem 0.25rem 0.25rem 1rem;
		border: 1.5px solid var(--blue-900);
		box-shadow: 0 2px 8px rgba(30, 91, 176, 0.15);
	}
	.followup-input {
		flex: 1;
		border: none;
		outline: none;
		font: inherit;
		font-size: 0.925rem;
		color: var(--text);
		background: transparent;
	}
	.btn-send {
		background: var(--blue-900);
		color: white;
		border: none;
		border-radius: 50%;
		width: 36px;
		height: 36px;
		font-size: 1rem;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.btn-send:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
	.btn-submit {
		width: 100%;
		padding: 0.875rem;
		background: linear-gradient(135deg, var(--blue-900), var(--blue-700));
		color: white;
		border: none;
		border-radius: var(--radius-sm);
		font: inherit;
		font-size: 0.95rem;
		font-weight: 700;
		cursor: pointer;
		box-shadow: 0 4px 14px rgba(30, 91, 176, 0.3);
		transition:
			opacity 0.15s,
			transform 0.1s;
	}
	.btn-submit:hover:not(:disabled) {
		opacity: 0.9;
		transform: translateY(-1px);
	}
	.btn-submit:disabled {
		opacity: 0.5;
		cursor: progress;
	}
	.ty-page {
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1.5rem;
		padding: 2rem 1rem;
		background: var(--bg);
		font-family: var(--font);
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
	.ty-card {
		width: min(100%, 480px);
		background: white;
		border: 1px solid var(--border);
		border-top: 5px solid var(--blue-900);
		border-radius: var(--radius-md);
		padding: 2.5rem 2rem;
		text-align: center;
		box-shadow: var(--shadow-md);
		display: flex;
		flex-direction: column;
		gap: 1rem;
		align-items: center;
	}
	.ty-icon {
		width: 52px;
		height: 52px;
		border-radius: 50%;
		background: var(--success-lt);
		border: 2px solid var(--success);
		color: var(--success);
		font-size: 1.4rem;
		font-weight: 700;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.ty-icon-neutral {
		background: var(--blue-50);
		border-color: var(--border);
		color: var(--muted);
	}
	.ty-card h1 {
		font-family: var(--font-display);
		font-size: 1.6rem;
		color: var(--blue-900);
		margin: 0;
	}
	.ty-card p {
		color: var(--text);
		font-size: 0.95rem;
		line-height: 1.6;
	}
	.ty-sub {
		color: var(--muted);
		font-size: 0.875rem;
	}
	.brand {
		font-size: 0.78rem;
		color: var(--muted);
	}
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(11, 44, 92, 0.4);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		z-index: 100;
		backdrop-filter: blur(4px);
	}
	.confirm-card {
		background: white;
		border-radius: 20px;
		padding: 2rem 1.75rem;
		max-width: 400px;
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 1rem;
		box-shadow: 0 24px 64px rgba(11, 44, 92, 0.2);
	}
	.confirm-card h2 {
		font-family: var(--font-display);
		font-size: 1.3rem;
		color: var(--blue-900);
	}
	.confirm-card p {
		color: var(--muted);
		font-size: 0.9rem;
	}
	.confirm-actions {
		display: flex;
		gap: 0.75rem;
		justify-content: flex-end;
		flex-wrap: wrap;
	}
	.btn-ghost {
		border: 1.5px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.55rem 1rem;
		background: white;
		color: var(--text);
		font: inherit;
		font-size: 0.875rem;
		cursor: pointer;
	}
	.btn-primary-sm {
		background: var(--blue-900);
		color: white;
		border: none;
		border-radius: var(--radius-sm);
		padding: 0.55rem 1.25rem;
		font: inherit;
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
	}
	.edit-card {
		background: white;
		border-radius: var(--radius-md);
		padding: 1rem 1.25rem;
		border: 1.5px solid var(--blue-700);
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		box-shadow: 0 2px 8px rgba(30, 91, 176, 0.15);
	}
	.edit-label {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0;
	}
	.edit-label strong {
		color: var(--text);
	}
	.btn-cancel-edit {
		background: none;
		border: none;
		color: var(--muted);
		font-size: 0.75rem;
		cursor: pointer;
		padding: 0;
	}
	.btn-cancel-edit:hover {
		color: var(--text);
	}
	.edit-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding-top: 0.25rem;
	}
	@media (max-width: 480px) {
		.scale-val {
			font-size: 2rem;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.dot-live,
		.typing span,
		.reconnect-spinner {
			animation: none;
		}
	}
</style>
