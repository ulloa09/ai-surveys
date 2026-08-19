<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import type { Question } from '$lib/types';
  import QuestionCard from '$lib/components/surveys/questions/QuestionCard.svelte';
  import QuestionForm from '$lib/components/surveys/questions/QuestionForm.svelte';

  let { data }: { data: PageData } = $props();

  const survey   = $derived(data.survey);
  const canEdit  = $derived(data.canEdit ?? false);
  const locked   = $derived(false); // se actualizará cuando haya respuestas

  // Lista local de preguntas para drag and drop optimista
  let questions = $state<Question[]>([]);

  $effect(() => {
    questions = data.questions ?? [];
  });

  // Sincronizar cuando cambia data.questions (tras reload)
  $effect(() => { questions = data.questions ?? []; });

  // Formulario
  let showForm    = $state(false);
  let editingQ    = $state<Question | null>(null);
  let formLoading = $state(false);
  let formError   = $state('');

  // Borrado
  let confirmDeleteID = $state('');
  let pendingDeleteID = $state('');

  // Reorder
  let dragFromIndex = $state<number | null>(null);
  let pendingOrder  = $state('');

  // Payload oculto para actions
  let createPayload = $state('');
  let updatePayload = $state('');
  let updateQID     = $state('');

  function openCreate() {
    editingQ  = null;
    showForm  = true;
    formError = '';
  }

  function openEdit(q: Question) {
    editingQ  = q;
    showForm  = true;
    formError = '';
  }

  function closeForm() {
    showForm  = false;
    editingQ  = null;
    formError = '';
  }

  // Drag and drop
  function handleDragStart(_e: DragEvent, index: number) {
    dragFromIndex = index;
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
  }

  function handleDrop(_e: DragEvent, toIndex: number) {
    if (dragFromIndex === null || dragFromIndex === toIndex) return;

    const reordered = [...questions];
    const [moved]   = reordered.splice(dragFromIndex, 1);
    reordered.splice(toIndex, 0, moved);
    questions     = reordered;
    dragFromIndex = null;

    // Guardamos el nuevo orden enviando al backend
    pendingOrder = JSON.stringify(reordered.map(q => q.id));
    setTimeout(() => {
      document.getElementById('form-reorder')?.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true })
      );
    }, 0);
  }
</script>

<svelte:head><title>Preguntas — {survey?.title ?? 'Encuesta'}</title></svelte:head>

<!-- Forms ocultos para actions -->
<form id="form-create" method="POST" action="?/createQuestion" style="display:none"
  use:enhance={() => {
    formLoading = true;
    return async ({ result, update }) => {
      formLoading = false;
      if (result.type === 'failure') {
        formError = (result.data as { createError?: string })?.createError ?? 'Error al crear';
      } else {
        formError = '';
        showForm = false;
        await update();
      }
    };
  }}
>
  <input type="hidden" name="payload" value={createPayload} />
</form>

<form id="form-update" method="POST" action="?/updateQuestion" style="display:none"
  use:enhance={() => {
    formLoading = true;
    return async ({ result, update }) => {
      formLoading = false;
      if (result.type === 'failure') {
        formError = (result.data as { updateError?: string })?.updateError ?? 'Error al actualizar';
      } else {
        formError = '';
        showForm = false;
        editingQ = null;
        await update();
      }
    };
  }}
>
  <input type="hidden" name="qid"     value={updateQID} />
  <input type="hidden" name="payload" value={updatePayload} />
</form>

<form id="form-delete" method="POST" action="?/deleteQuestion" style="display:none"
  use:enhance={() => {
    return async ({ result, update }) => {
      pendingDeleteID = '';
      confirmDeleteID = '';
      if (result.type === 'failure') {
        formError = (result.data as { deleteError?: string })?.deleteError ?? 'Error al eliminar';
      } else {
        await update();
      }
    };
  }}
>
  <input type="hidden" name="qid" value={pendingDeleteID} />
</form>

<form id="form-reorder" method="POST" action="?/reorderQuestions" style="display:none"
  use:enhance={() => {
    return async ({ result, update }) => {
      if (result.type === 'failure') {
        formError = (result.data as { reorderError?: string })?.reorderError ?? 'No se pudo reordenar';
      }
      await update();
    };
  }}
>
  <input type="hidden" name="order" value={pendingOrder} />
</form>

<div class="page">

  <!-- Banda institucional de título -->
  <div class="page-header">
    <div class="page-header-inner">
      <div class="header-left">
        <a href="/dashboard/surveys/{survey?.id}" class="back-link">← {survey?.title}</a>
        <h1 class="page-title">Preguntas</h1>
        <p class="page-sub">{questions.length} pregunta(s)</p>
      </div>
      {#if canEdit && !showForm}
        <button class="btn-band" onclick={openCreate}>+ Agregar pregunta</button>
      {/if}
    </div>
  </div>

  <div class="page-content">

  <!-- Aviso si está bloqueado -->
  {#if locked}
    <div class="locked-banner">
      Esta encuesta ya tiene respuestas. Las preguntas no se pueden modificar.
    </div>
  {/if}

  <!-- Error general -->
  {#if formError && !showForm}
    <div class="action-error">
      {formError}
      <button onclick={() => formError = ''}>✕</button>
    </div>
  {/if}

  <!-- Formulario crear / editar -->
  {#if showForm && canEdit}
    <div class="form-card">
      <h2 class="card-title">
        {editingQ ? 'Editar pregunta' : 'Nueva pregunta'}
      </h2>
      <QuestionForm
        question={editingQ}
        loading={formLoading}
        error={formError}
        onCancel={closeForm}
        onSubmit={(qdata) => {
          const payload = JSON.stringify({
            type:        qdata.type,
            text:        qdata.text,
            required:    qdata.required,
            ai_followup: qdata.ai_followup,
            options:     qdata.options ?? undefined
          });

          if (editingQ) {
            updateQID     = editingQ.id;
            updatePayload = payload;
            setTimeout(() => {
              document.getElementById('form-update')?.dispatchEvent(
                new Event('submit', { bubbles: true, cancelable: true })
              );
            }, 0);
          } else {
            createPayload = payload;
            setTimeout(() => {
              document.getElementById('form-create')?.dispatchEvent(
                new Event('submit', { bubbles: true, cancelable: true })
              );
            }, 0);
          }
        }}
      />
    </div>
  {/if}

  <!-- Modal confirmacion borrado -->
  {#if confirmDeleteID}
    <div class="confirm-overlay">
      <div class="confirm-card">
        <h3>¿Eliminar esta pregunta?</h3>
        <p>Esta acción no se puede deshacer.</p>
        <div class="confirm-actions">
          <button class="btn-cancel" onclick={() => confirmDeleteID = ''}>Cancelar</button>
          <button class="btn-danger" onclick={() => {
            pendingDeleteID = confirmDeleteID;
            setTimeout(() => {
              document.getElementById('form-delete')?.dispatchEvent(
                new Event('submit', { bubbles: true, cancelable: true })
              );
            }, 0);
          }}>
            Sí, eliminar
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Lista de preguntas -->
  <div class="questions-list" role="list">
    {#if questions.length === 0}
      <div class="empty">
        <p class="empty-title">No hay preguntas todavía</p>
        {#if canEdit}
          <p class="empty-desc">Agrega la primera pregunta usando el botón de arriba.</p>
        {/if}
      </div>
    {:else}
      {#each questions as q, i}
        <QuestionCard
          question={q}
          index={i}
          canEdit={canEdit}
          {locked}
          onEdit={openEdit}
          onDelete={(id) => confirmDeleteID = id}
          ondragstart={(e) => handleDragStart(e, i)}
          ondragover={handleDragOver}
          ondrop={(e) => handleDrop(e, i)}
        />
      {/each}
    {/if}
  </div>

  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  .page-header {
    background: var(--blue-700);
    padding: 2.25rem 2rem;
  }

  .page-header-inner {
    max-width: var(--container);
    margin: 0 auto;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .page-content {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    padding: 2rem;
    width: 100%;
    max-width: var(--container);
    margin: 0 auto;
  }

  .header-left { display: flex; flex-direction: column; gap: 0.25rem; }

  .back-link {
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.8);
    text-decoration: none;
  }

  .back-link:hover { color: #fff; }

  .page-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: #fff;
    margin: 0;
  }

  .page-sub { font-size: 0.875rem; color: rgba(255, 255, 255, 0.8); margin: 0; }

  .btn-band {
    background: #fff;
    color: var(--blue-900);
    border: 1px solid #fff;
    border-radius: var(--radius-pill);
    padding: 0.5rem 1.25rem;
    font-family: var(--font-display);
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .btn-band:hover { background: var(--blue-50); }

  .locked-banner {
    background: var(--warning-lt);
    border: 1px solid #fde047;
    border-radius: var(--radius-sm);
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
    color: #854d0e;
  }

  .action-error {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    background: #fef2f2;
    border: 1px solid #fca5a5;
    border-radius: var(--radius-sm);
    padding: 0.6rem 0.75rem;
    font-size: 0.8rem;
    color: #dc2626;
  }

  .action-error button {
    background: none;
    border: none;
    color: #dc2626;
    cursor: pointer;
    font-size: 0.9rem;
    padding: 0;
  }

  .form-card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.75rem;
    box-shadow: 0 2px 8px rgba(0,0,0,0.06);
  }

  .card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
    margin: 0 0 1.25rem;
  }

  .questions-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .empty {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 3rem 2rem;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
  }

  .empty-title { font-size: 1rem; font-weight: 600; color: var(--text); margin: 0; }
  .empty-desc  { font-size: 0.875rem; color: var(--muted); margin: 0; }

  /* Modal */
  .confirm-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }

  .confirm-card {
    background: var(--white);
    border-radius: var(--radius-lg);
    padding: 2rem;
    max-width: 380px;
    width: 90%;
    box-shadow: var(--shadow-lg);
  }

  .confirm-card h3 {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
    margin: 0 0 0.5rem;
  }

  .confirm-card p {
    font-size: 0.875rem;
    color: var(--muted);
    margin: 0 0 1.5rem;
    line-height: 1.6;
  }

  .confirm-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
  }

  .btn-cancel {
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
    color: var(--muted);
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-cancel:hover { border-color: var(--purple); color: var(--purple); }

  .btn-danger {
    background: #dc2626;
    color: white;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .btn-danger:hover { opacity: 0.85; }
</style>