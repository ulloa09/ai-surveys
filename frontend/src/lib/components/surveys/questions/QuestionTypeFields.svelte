<script lang="ts">
  import type { QuestionType, ChoiceOption } from '$lib/types';
  import { QUESTION_TYPE_DESCRIPTIONS } from '$lib/types';

  let {
    type,
    mode = 'conversational',
    options = null,
    disabled = false,
    onChange = (_opts: unknown) => {}
  }: {
    type: QuestionType;
    mode?: 'conversational' | 'form' | 'prompt_only';
    options?: unknown;
    disabled?: boolean;
    onChange?: (opts: unknown) => void;
  } = $props();

  // La descripción de open_ended promete que "la IA puede hacer seguimiento",
  // cosa que en Modo A no ocurre: ahí la encuesta son las preguntas del admin y
  // nada más.
  const description = $derived(
    type === 'open_ended' && mode === 'form'
      ? 'El participante escribe libremente.'
      : QUESTION_TYPE_DESCRIPTIONS[type]
  );

  // Genera un value único basado en el label o un ID incremental
  function genValue(label: string, index: number): string {
    const slug = label.trim().toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_]/g, '');
    return slug || String(index + 1);
  }

  // --- single_choice y multi_choice ---
  let choices = $state<ChoiceOption[]>(
    (options as any)?.choices ?? [{ label: '', value: '' }, { label: '', value: '' }]
  );

  function addChoice() {
    choices = [...choices, { label: '', value: '' }];
    emitChoices();
  }

  function removeChoice(i: number) {
    choices = choices.filter((_, idx) => idx !== i);
    emitChoices();
  }

  function updateChoiceLabel(i: number, val: string) {
    choices = choices.map((c, idx) => idx === i ? { label: val, value: genValue(val, i) } : c);
    emitChoices();
  }

  function emitChoices() {
    onChange({ choices });
  }

  // --- linear_scale ---
  let scaleMin      = $state<number>((options as any)?.min ?? 1);
  let scaleMax      = $state<number>((options as any)?.max ?? 5);
  let scaleMinLabel = $state<string>((options as any)?.min_label ?? '');
  let scaleMaxLabel = $state<string>((options as any)?.max_label ?? '');

  function emitScale() {
    onChange({
      min: scaleMin,
      max: scaleMax,
      min_label: scaleMinLabel || undefined,
      max_label: scaleMaxLabel || undefined
    });
  }

  // --- ranking ---
  let rankingItems = $state<ChoiceOption[]>(
    (options as any)?.items ?? [{ label: '', value: '' }, { label: '', value: '' }]
  );

  function addRankingItem() {
    rankingItems = [...rankingItems, { label: '', value: '' }];
    emitRanking();
  }

  function removeRankingItem(i: number) {
    rankingItems = rankingItems.filter((_, idx) => idx !== i);
    emitRanking();
  }

  function updateRankingLabel(i: number, val: string) {
    rankingItems = rankingItems.map((item, idx) => idx === i ? { label: val, value: genValue(val, i) } : item);
    emitRanking();
  }

  function emitRanking() {
    onChange({ items: rankingItems });
  }

  // --- matrix ---
  let matrixRows    = $state<string[]>((options as any)?.rows    ?? ['', '']);
  let matrixColumns = $state<string[]>((options as any)?.columns ?? ['', '']);

  function addMatrixRow()    { matrixRows    = [...matrixRows, '']; emitMatrix(); }
  function addMatrixColumn() { matrixColumns = [...matrixColumns, '']; emitMatrix(); }

  function removeMatrixRow(i: number)    { matrixRows    = matrixRows.filter((_, idx) => idx !== i); emitMatrix(); }
  function removeMatrixColumn(i: number) { matrixColumns = matrixColumns.filter((_, idx) => idx !== i); emitMatrix(); }

  function updateMatrixRow(i: number, val: string)    { matrixRows    = matrixRows.map((r, idx) => idx === i ? val : r); emitMatrix(); }
  function updateMatrixColumn(i: number, val: string) { matrixColumns = matrixColumns.map((c, idx) => idx === i ? val : c); emitMatrix(); }

  function emitMatrix() {
    onChange({ rows: matrixRows, columns: matrixColumns });
  }
</script>

<div class="type-fields">
  <p class="type-desc">{description}</p>

  {#if type === 'open_ended' || type === 'true_false'}
    <p class="no-options">Este tipo de pregunta no requiere opciones adicionales.</p>

  {:else if type === 'single_choice' || type === 'multi_choice'}
    <div class="field-group">
      <p class="field-label">Opciones <span class="required">*</span></p>
      <div class="options-list">
        {#each choices as choice, i}
          <div class="option-row">
            <input
              type="text"
              placeholder={`Opción ${i + 1}`}
              value={choice.label}
              oninput={(e) => updateChoiceLabel(i, (e.target as HTMLInputElement).value)}
              disabled={disabled}
            />
            {#if choices.length > 2}
              <button type="button" class="btn-remove" onclick={() => removeChoice(i)} disabled={disabled}>✕</button>
            {/if}
          </div>
        {/each}
      </div>
      <button type="button" class="btn-add" onclick={addChoice} disabled={disabled}>
        + Agregar opción
      </button>
    </div>

  {:else if type === 'linear_scale'}
    <div class="field-group">
      <p class="field-label">Rango <span class="required">*</span></p>
      <div class="scale-row">
        <label class="scale-field">
          <span>Mínimo</span>
          <input type="number" bind:value={scaleMin} oninput={emitScale} disabled={disabled} min="0" max="99" />
        </label>
        <span class="scale-sep">→</span>
        <label class="scale-field">
          <span>Máximo</span>
          <input type="number" bind:value={scaleMax} oninput={emitScale} disabled={disabled} min="1" max="100" />
        </label>
      </div>
      <div class="scale-labels">
        <label class="scale-field">
          <span>Etiqueta mínimo (opcional)</span>
          <input type="text" placeholder="Ej. Muy malo" bind:value={scaleMinLabel} oninput={emitScale} disabled={disabled} />
        </label>
        <label class="scale-field">
          <span>Etiqueta máximo (opcional)</span>
          <input type="text" placeholder="Ej. Excelente" bind:value={scaleMaxLabel} oninput={emitScale} disabled={disabled} />
        </label>
      </div>
    </div>

  {:else if type === 'ranking'}
    <div class="field-group">
      <p class="field-label">Elementos a ordenar <span class="required">*</span></p>
      <div class="options-list">
        {#each rankingItems as item, i}
          <div class="option-row">
            <input
              type="text"
              placeholder={`Elemento ${i + 1}`}
              value={item.label}
              oninput={(e) => updateRankingLabel(i, (e.target as HTMLInputElement).value)}
              disabled={disabled}
            />
            {#if rankingItems.length > 2}
              <button type="button" class="btn-remove" onclick={() => removeRankingItem(i)} disabled={disabled}>✕</button>
            {/if}
          </div>
        {/each}
      </div>
      <button type="button" class="btn-add" onclick={addRankingItem} disabled={disabled}>
        + Agregar elemento
      </button>
    </div>

  {:else if type === 'matrix'}
    <div class="field-group">
      <p class="field-label">Filas (elementos a evaluar) <span class="required">*</span></p>
      <div class="options-list">
        {#each matrixRows as row, i}
          <div class="option-row">
            <input
              type="text"
              placeholder={`Fila ${i + 1}`}
              value={row}
              oninput={(e) => updateMatrixRow(i, (e.target as HTMLInputElement).value)}
              disabled={disabled}
            />
            {#if matrixRows.length > 1}
              <button type="button" class="btn-remove" onclick={() => removeMatrixRow(i)} disabled={disabled}>✕</button>
            {/if}
          </div>
        {/each}
      </div>
      <button type="button" class="btn-add" onclick={addMatrixRow} disabled={disabled}>+ Agregar fila</button>
    </div>

    <div class="field-group">
      <p class="field-label">Columnas (escala de evaluación) <span class="required">*</span></p>
      <div class="options-list">
        {#each matrixColumns as col, i}
          <div class="option-row">
            <input
              type="text"
              placeholder={`Col ${i + 1}`}
              value={col}
              oninput={(e) => updateMatrixColumn(i, (e.target as HTMLInputElement).value)}
              disabled={disabled}
            />
            {#if matrixColumns.length > 2}
              <button type="button" class="btn-remove" onclick={() => removeMatrixColumn(i)} disabled={disabled}>✕</button>
            {/if}
          </div>
        {/each}
      </div>
      <button type="button" class="btn-add" onclick={addMatrixColumn} disabled={disabled}>+ Agregar columna</button>
    </div>
  {/if}
</div>

<style>
  .type-fields { display: flex; flex-direction: column; gap: 1rem; }
  .type-desc { font-size: 0.8rem; color: var(--muted); background: var(--bg); border-radius: var(--radius-sm); padding: 0.5rem 0.75rem; margin: 0; line-height: 1.5; }
  .no-options { font-size: 0.8rem; color: var(--muted); margin: 0; font-style: italic; }
  .field-group { display: flex; flex-direction: column; gap: 0.5rem; }
  .field-label { font-size: 0.75rem; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; margin: 0; }
  .required { color: #dc2626; }
  .options-list { display: flex; flex-direction: column; gap: 0.4rem; }
  .option-row { display: flex; gap: 0.5rem; align-items: center; }
  .option-row input { flex: 1; background: var(--white); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.4rem 0.6rem; font-size: 0.8rem; color: var(--text); outline: none; transition: border-color 0.15s; }
  .option-row input:focus { border-color: var(--purple); }
  .btn-remove { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.3rem 0.5rem; font-size: 0.75rem; color: var(--muted); cursor: pointer; transition: all 0.15s; flex-shrink: 0; }
  .btn-remove:hover { border-color: #dc2626; color: #dc2626; }
  .btn-add { background: none; border: 1px dashed var(--border); border-radius: var(--radius-sm); padding: 0.4rem 0.75rem; font-size: 0.8rem; color: var(--muted); cursor: pointer; transition: all 0.15s; text-align: left; align-self: flex-start; }
  .btn-add:hover { border-color: var(--purple); color: var(--purple); }
  .scale-row, .scale-labels { display: flex; gap: 1rem; align-items: center; flex-wrap: wrap; }
  .scale-sep { color: var(--muted); font-size: 1rem; }
  .scale-field { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.75rem; color: var(--muted); text-transform: none; letter-spacing: 0; font-weight: 500; flex: 1; min-width: 120px; }
  .scale-field input { background: var(--white); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.4rem 0.6rem; font-size: 0.875rem; color: var(--text); outline: none; transition: border-color 0.15s; width: 100%; }
  .scale-field input:focus { border-color: var(--purple); }
</style>