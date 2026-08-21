<script lang="ts">
  import type { Survey } from '$lib/types';
  import { STATUS_LABELS } from '$lib/types';

  let {
    survey,
    canEdit = false,
    canDelete = false,
    canDuplicate = false,
    onDelete = (_id: string) => {},
    onDuplicate = (_id: string) => {}
  }: {
    survey: Survey;
    canEdit?: boolean;
    canDelete?: boolean;
    canDuplicate?: boolean;
    onDelete?: (id: string) => void;
    onDuplicate?: (id: string) => void;
  } = $props();

  function formatDate(dateStr: string) {
    return new Date(dateStr).toLocaleDateString('es-MX', {
      year: 'numeric', month: 'short', day: 'numeric'
    });
  }

  const statusClass: Record<string, string> = {
    draft:     'status-draft',
    open:      'status-open',
    closed:    'status-closed',
    analysing: 'status-analysing',
    complete:  'status-complete',
    failed:    'status-failed',
    archived:  'status-archived'
  };

  // Una encuesta en draft nunca tuvo respuestas: mostrar "0 respuestas, 0% de
  // completado" sería ruido, así que la fila de stats solo aparece cuando hay
  // al menos una respuesta.
  const stats = $derived(survey.stats);
  const showStats = $derived((stats?.response_count ?? 0) > 0);
  const completionPct = $derived(Math.round((stats?.completion_rate ?? 0) * 100));
</script>

<div class="card" class:card--open={survey.status === 'open'}>

  {#if canDuplicate || (canDelete && survey.status === 'draft')}
    <div class="card-actions">
      {#if canDuplicate}
        <button class="action-btn" title="Duplicar" onclick={() => onDuplicate(survey.id)}>⧉</button>
      {/if}
      {#if canDelete && survey.status === 'draft'}
        <button class="action-btn action-delete" title="Eliminar" onclick={() => onDelete(survey.id)}>✕</button>
      {/if}
    </div>
  {/if}

  <span class="status-badge {statusClass[survey.status] ?? 'status-draft'}">
    {STATUS_LABELS[survey.status as keyof typeof STATUS_LABELS] ?? survey.status}
  </span>

  <a href="/dashboard/surveys/{survey.id}" class="card-title">
    {survey.title}
  </a>

  {#if survey.description}
    <p class="card-desc">{survey.description}</p>
  {/if}

  {#if survey.team_name}
    <div class="team-badges">
      <span class="team-badge">{survey.team_name}</span>
    </div>
  {/if}

  {#if showStats && stats}
    <div class="card-stats">
      <span class="stat">
        <strong>{stats.response_count}</strong>
        {stats.response_count === 1 ? 'respuesta' : 'respuestas'}
      </span>
      <span class="stat">
        <strong>{completionPct}%</strong>
        completado
      </span>
    </div>
  {/if}

  <div class="card-footer">
    <span class="card-owner">
      <span class="avatar">{(survey.owner_name ?? '?')[0].toUpperCase()}</span>
      {survey.owner_name ?? 'Sin dueño'}
    </span>
    <span class="card-date">{formatDate(survey.created_at)}</span>
  </div>

</div>

<style>
  .card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 1.375rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    box-shadow: var(--shadow-sm);
    transition: box-shadow 0.15s, border-color 0.15s;
    position: relative;
    cursor: default;
  }

  .card:hover {
    box-shadow: var(--shadow-md);
    border-color: var(--blue-400);
  }

  .card--open {
    border-left: 3px solid var(--blue-400);
  }

  .card-actions {
    position: absolute;
    top: 1rem;
    right: 1rem;
    display: flex;
    gap: 0.3rem;
  }

  .action-btn {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.2rem 0.5rem;
    font-size: 0.8rem;
    color: var(--muted);
    cursor: pointer;
    transition: all 0.15s;
    line-height: 1;
  }

  .action-btn:hover      { border-color: var(--blue-400); color: var(--blue-900); background: var(--blue-50); }
  .action-delete:hover   { border-color: var(--danger); color: var(--danger); background: var(--danger-lt); }

  .status-badge {
    font-family: var(--font-display);
    font-size: 0.62rem;
    font-weight: 600;
    padding: 0.2rem 0.6rem;
    border-radius: var(--radius-pill);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    white-space: nowrap;
    align-self: flex-start;
  }

  .status-draft     { background: var(--blue-50);      color: var(--muted);     border: 1px solid var(--border); }
  .status-open      { background: var(--success-lt);   color: var(--success);   border: 1px solid var(--success); }
  .status-closed    { background: var(--warning-lt);   color: var(--warning);   border: 1px solid var(--warning); }
  .status-analysing { background: var(--blue-50);      color: var(--blue-600);  border: 1px solid var(--blue-400); }
  .status-complete  { background: var(--blue-100);     color: var(--blue-900);  border: 1px solid var(--blue-900); }
  .status-failed    { background: var(--danger-lt);    color: var(--danger);    border: 1px solid var(--danger); }
  .status-archived  { background: #f1f2f4;             color: var(--muted);     border: 1px solid var(--border); }

  /* ── Stats (#16) ─────────────────────────────────────────────────── */
  .card-stats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem 1rem;
  }

  .card-stats .stat {
    font-size: 0.775rem;
    color: var(--muted);
  }

  .card-stats strong {
    color: var(--text);
    font-weight: 700;
  }

  .card-title {
    font-family: var(--font-display);
    font-size: 1rem;
    font-weight: 600;
    color: var(--blue-900);
    text-decoration: none;
    line-height: 1.35;
    letter-spacing: -0.01em;
    padding-right: 3.5rem;
  }

  .card-title:hover { color: var(--blue-500); text-decoration: underline; }

  .card-desc {
    font-size: 0.8125rem;
    color: var(--muted);
    line-height: 1.55;
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .card-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: 0.5rem;
    border-top: 1px solid var(--border);
    margin-top: auto;
  }

  .card-owner {
    font-size: 0.775rem;
    color: var(--muted);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .avatar {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--blue-900);
    color: #fff;
    font-family: var(--font-display);
    font-size: 0.6rem;
    font-weight: 600;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .card-date { font-size: 0.775rem; color: var(--muted); }

  .team-badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .team-badge {
    font-size: 0.7rem;
    font-weight: 600;
    color: var(--blue-600);
    background: var(--blue-50);
    border: 1px solid var(--blue-200);
    border-radius: var(--radius-pill);
    padding: 0.1rem 0.55rem;
  }
</style>
