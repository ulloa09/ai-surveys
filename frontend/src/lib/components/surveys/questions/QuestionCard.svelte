<script lang="ts">
  import type { Question } from '$lib/types';
  import { QUESTION_TYPE_LABELS } from '$lib/types';

  let {
    question,
    index,
    mode = 'conversational',
    canEdit = false,
    locked = false,
    onEdit = (_q: Question) => {},
    onDelete = (_id: string) => {},
    ondragstart = (_e: DragEvent) => {},
    ondragover = (_e: DragEvent) => {},
    ondrop = (_e: DragEvent) => {},
    ondragend = (_e: DragEvent) => {}
  }: {
    question: Question;
    index: number;
    mode?: 'conversational' | 'form' | 'prompt_only';
    canEdit?: boolean;
    locked?: boolean;
    onEdit?: (q: Question) => void;
    onDelete?: (id: string) => void;
    ondragstart?: (e: DragEvent) => void;
    ondragover?: (e: DragEvent) => void;
    ondrop?: (e: DragEvent) => void;
    ondragend?: (e: DragEvent) => void;
  } = $props();

  let isDraggingOver = $state(false);

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    isDraggingOver = true;
    ondragover(e);
  }

  function handleDragLeave() {
    isDraggingOver = false;
  }

  function handleDrop(e: DragEvent) {
    isDraggingOver = false;
    ondrop(e);
  }
</script>

<div
  class="card"
  class:locked
  class:drag-over={isDraggingOver}
  draggable={canEdit && !locked}
  {ondragstart}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
  {ondragend}
  role="listitem"
>
  <!-- Drag handle -->
  <span
    class="drag-handle"
    class:active={canEdit && !locked}
    title={canEdit && !locked ? 'Arrastra para reordenar' : ''}
  >⠿</span>

  <!-- Número -->
  <span class="q-num">{index + 1}</span>

  <!-- Contenido -->
  <div class="q-body">
    <p class="q-text">{question.text}</p>
    <div class="q-meta">
      <span class="q-type">{QUESTION_TYPE_LABELS[question.type]}</span>
      {#if question.required}
        <span class="badge-required">Obligatoria</span>
      {/if}
      <!-- En modo 'form' no hay seguimiento, así que el badge no se muestra ni
           siquiera si la pregunta quedó con ai_followup = true de otro modo. -->
      {#if question.ai_followup && mode !== 'form'}
        <span class="badge-ai">IA</span>
      {/if}
    </div>
  </div>

  <!-- Acciones -->
  {#if canEdit}
    <div class="q-actions">
      {#if locked}
        <span class="locked-msg">Bloqueada</span>
      {:else}
        <button class="action-btn" onclick={() => onEdit(question)}>Editar</button>
        <button class="action-btn action-delete" onclick={() => onDelete(question.id)}>Eliminar</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1rem 1.25rem;
    display: flex;
    align-items: center;
    gap: 0.875rem;
    transition: box-shadow 0.15s, border-color 0.15s;
    user-select: none;
  }

  .card[draggable="true"] { cursor: grab; }
  .card[draggable="true"]:active { cursor: grabbing; }
  .card.drag-over {
    border-color: var(--purple);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--purple) 20%, transparent);
  }
  .card.locked { opacity: 0.7; }

  .drag-handle {
    font-size: 1.1rem;
    color: var(--border);
    flex-shrink: 0;
    line-height: 1;
    cursor: default;
  }

  .drag-handle.active {
    color: var(--muted);
    cursor: grab;
  }

  .q-num {
    background: var(--purple);
    color: white;
    font-size: 0.75rem;
    font-weight: 700;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .q-body {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    min-width: 0;
  }

  .q-text {
    font-size: 0.95rem;
    color: var(--text);
    margin: 0;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .q-meta {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .q-type {
    font-size: 0.8rem;
    color: var(--muted);
  }

  .badge-required, .badge-ai {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    border-radius: 20px;
    text-transform: uppercase;
    letter-spacing: 0.3px;
  }

  .badge-required { background: #f1f5f9; color: #64748b; }
  .badge-ai { background: color-mix(in srgb, var(--purple) 12%, transparent); color: var(--purple); }

  .q-actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-shrink: 0;
  }

  .action-btn {
    background: var(--purple);
    color: white;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.35rem 0.875rem;
    font-size: 0.825rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .action-btn:hover { opacity: 0.85; }

  .action-delete {
    background: white;
    color: #991b1b;
    border: 1.5px solid #fca5a5;
  }

  .action-delete:hover { background: #fef2f2; opacity: 1; }

  .locked-msg {
    font-size: 0.8rem;
    color: var(--muted);
    white-space: nowrap;
  }
</style>