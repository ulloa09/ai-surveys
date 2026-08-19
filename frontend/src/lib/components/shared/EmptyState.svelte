<script lang="ts">
  // Estado vacío institucional: sin iconos decorativos — solo una regla
  // azul y tipografía.
  let {
    title = 'No hay nada aquí todavía',
    description = '',
    actionLabel = '',
    onAction = () => {}
  }: {
    title?: string;
    description?: string;
    actionLabel?: string;
    onAction?: () => void;
  } = $props();
</script>

<div class="empty">
  <span class="empty-rule" aria-hidden="true"></span>
  <h3 class="empty-title">{title}</h3>
  {#if description}
    <p class="empty-desc">{description}</p>
  {/if}
  {#if actionLabel}
    <button class="empty-btn" onclick={onAction}>{actionLabel}</button>
  {/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 3rem 2rem;
    text-align: center;
    gap: 0.5rem;
  }

  .empty-rule {
    width: 48px;
    height: 3px;
    background: var(--blue-400);
    margin-bottom: 1rem;
  }

  .empty-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--blue-900);
    margin: 0;
  }

  .empty-desc {
    font-size: 0.875rem;
    color: var(--muted);
    margin: 0;
    max-width: 320px;
    line-height: 1.6;
  }

  .empty-btn {
    margin-top: 1rem;
    background: var(--blue-900);
    color: #fff;
    border: 1px solid var(--blue-900);
    border-radius: var(--radius-pill);
    padding: 0.5rem 1.25rem;
    font-family: var(--font-display);
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.15s;
  }

  .empty-btn:hover { background: var(--blue-700); border-color: var(--blue-700); }
</style>
