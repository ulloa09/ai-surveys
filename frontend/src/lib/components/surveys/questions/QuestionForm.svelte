<script lang="ts">
  import type { Question, QuestionType } from '$lib/types';
  import { QUESTION_TYPE_LABELS } from '$lib/types';
  import QuestionTypeFields from './QuestionTypeFields.svelte';

  let {
    question = null,
    mode = 'conversational',
    loading = false,
    error = '',
    onSubmit = (_data: Partial<Question>) => {},
    onCancel = () => {}
  }: {
    question?: Question | null;
    mode?: 'conversational' | 'form' | 'prompt_only';
    loading?: boolean;
    error?: string;
    onSubmit?: (data: Partial<Question>) => void;
    onCancel?: () => void;
  } = $props();

  const isEdit = $derived(!!question);

  // Modo A ("preguntas fijas"): la encuesta es el cuestionario del admin y nada
  // más, así que el toggle de seguimiento no se ofrece — no habría nada que
  // decidir. El backend lo hace cumplir de todos modos (ver followupsAllowed).
  const followupsAllowed = $derived(mode !== 'form');

  const questionTypes: QuestionType[] = [
    'open_ended', 'single_choice', 'multi_choice',
    'true_false', 'linear_scale', 'ranking', 'matrix'
  ];

  let selectedType = $state<QuestionType>(question?.type ?? 'open_ended');
  let text         = $state(question?.text ?? '');
  let required     = $state(question?.required ?? true);
  let aiFollowup   = $state(question?.ai_followup ?? (question?.type === 'open_ended' ? true : false));
  let options      = $state<unknown>(question?.options ?? null);

  // Cuando cambia el tipo, actualiza el default de ai_followup
  $effect(() => {
    if (!isEdit) {
      aiFollowup = selectedType === 'open_ended';
      options = null;
    }
  });

  function handleOptionsChange(opts: unknown) {
    options = opts;
  }

  function handleSubmit(e: Event) {
    e.preventDefault();
    onSubmit({
      type: selectedType,
      text,
      required,
      // En modo 'form' se guarda siempre en false: así el dato coincide con lo
      // que realmente pasa, en vez de quedar un toggle prendido que el engine
      // ignora.
      ai_followup: followupsAllowed && aiFollowup,
      options: options as Question['options']
    });
  }
</script>

<form class="form" onsubmit={handleSubmit}>

  {#if !isEdit}
    <div class="field">
      <span class="field-label">Tipo de pregunta <span class="required">*</span></span>
      <div class="type-grid" role="radiogroup" aria-label="Tipo de pregunta">
        {#each questionTypes as qtype}
          <button
            type="button"
            class="type-btn"
            class:selected={selectedType === qtype}
            onclick={() => selectedType = qtype}
            disabled={loading}
          >
            {QUESTION_TYPE_LABELS[qtype]}
          </button>
        {/each}
      </div>
    </div>
  {:else}
    <p class="edit-type-info">
      Tipo: <strong>{QUESTION_TYPE_LABELS[selectedType]}</strong>
      <span class="muted">(no se puede cambiar el tipo al editar)</span>
    </p>
  {/if}

  <div class="field">
    <label for="q-text">Texto de la pregunta <span class="required">*</span></label>
    <textarea
      id="q-text"
      placeholder="Escribe la pregunta aquí…"
      bind:value={text}
      rows="2"
      required
      disabled={loading}
    ></textarea>
  </div>

  <!-- Campos específicos por tipo -->
  <QuestionTypeFields
    type={selectedType}
    {mode}
    options={question?.options}
    disabled={loading}
    onChange={handleOptionsChange}
  />

  <!-- Toggles -->
  <div class="toggles">
    <label class="toggle">
      <input type="checkbox" bind:checked={required} disabled={loading} />
      <span class="toggle-label">
        <strong>Obligatoria</strong>
        <small>El participante debe responder esta pregunta para continuar.</small>
      </span>
    </label>

    {#if followupsAllowed}
      <label class="toggle">
        <input type="checkbox" bind:checked={aiFollowup} disabled={loading} />
        <span class="toggle-label">
          <strong>Seguimiento de IA</strong>
          <small>La IA hará preguntas de seguimiento basadas en la respuesta.</small>
        </span>
      </label>
    {/if}
  </div>

  {#if error}
    <p class="form-error">{error}</p>
  {/if}

  <div class="form-footer">
    <button type="button" class="btn-cancel" onclick={onCancel} disabled={loading}>
      Cancelar
    </button>
    <button type="submit" class="btn-primary" disabled={loading || !text.trim()}>
      {loading ? 'Guardando…' : isEdit ? 'Guardar cambios' : 'Agregar pregunta'}
    </button>
  </div>

</form>

<style>
  .form { display: flex; flex-direction: column; gap: 1.25rem; }

  .field { display: flex; flex-direction: column; gap: 0.4rem; }

  label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .required { color: #dc2626; }

  textarea {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    font-size: 0.875rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
    width: 100%;
    resize: vertical;
  }

  textarea:focus { border-color: var(--purple); }

  .type-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .type-btn {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.4rem 0.75rem;
    font-size: 0.8rem;
    color: var(--muted);
    cursor: pointer;
    transition: all 0.15s;
  }

  .type-btn:hover { border-color: var(--purple); color: var(--purple); }

  .type-btn.selected {
    background: color-mix(in srgb, var(--purple) 12%, transparent);
    border-color: var(--purple);
    color: var(--purple);
    font-weight: 600;
  }

  .edit-type-info {
    font-size: 0.875rem;
    color: var(--text);
    margin: 0;
  }

  .muted { color: var(--muted); font-weight: 400; }

  .toggles { display: flex; flex-direction: column; gap: 0.75rem; }

  .toggle {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    cursor: pointer;
    text-transform: none;
    letter-spacing: 0;
    font-size: 1rem;
    font-weight: 400;
  }

  .toggle input[type="checkbox"] {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
    margin-top: 2px;
    accent-color: var(--purple);
    cursor: pointer;
  }

  .toggle-label { display: flex; flex-direction: column; gap: 0.2rem; }

  .toggle-label strong { font-size: 0.875rem; color: var(--text); }

  .toggle-label small {
    font-size: 0.775rem;
    color: var(--muted);
    line-height: 1.4;
  }

  .form-error {
    font-size: 0.8rem;
    color: #dc2626;
    background: #fef2f2;
    border: 1px solid #fca5a5;
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    margin: 0;
  }

  .form-footer {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.75rem;
    padding-top: 0.5rem;
    border-top: 1px solid var(--border);
  }

  .btn-cancel {
    background: none;
    border: none;
    color: var(--muted);
    font-size: 0.875rem;
    cursor: pointer;
    padding: 0.5rem;
    transition: color 0.15s;
  }

  .btn-cancel:hover { color: var(--text); }

  .btn-primary {
    background: var(--purple);
    color: white;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.5rem 1.5rem;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-primary:not(:disabled):hover { opacity: 0.85; }
</style>
