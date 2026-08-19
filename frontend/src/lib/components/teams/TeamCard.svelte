<script lang="ts">
  import type { Team } from '$lib/types';

  let {
    team,
    canManage = false
  }: {
    team: Team;
    canManage?: boolean;
  } = $props();

  function formatDate(dateStr: string) {
    return new Date(dateStr).toLocaleDateString('es-MX', {
      year: 'numeric', month: 'short', day: 'numeric'
    });
  }
</script>

<a href="/dashboard/teams/{team.id}" class="card">
  <div class="card-header">
    <span class="card-icon" aria-hidden="true"></span>
    <span class="card-name">{team.name}</span>
    {#if canManage}
      <span class="badge-manage">Gestionar →</span>
    {/if}
  </div>
  <p class="card-date">Creado el {formatDate(team.created_at)}</p>
</a>

<style>
  .card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.25rem;
    text-decoration: none;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    transition: all 0.15s;
    box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  }

  .card:hover {
    border-color: var(--purple);
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(76,65,99,0.12);
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .card-icon {
    width: 4px;
    height: 18px;
    background: var(--blue-400);
    flex-shrink: 0;
  }

  .card-name {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--text);
    flex: 1;
  }

  .badge-manage {
    font-size: 0.7rem;
    color: var(--purple);
    font-weight: 600;
  }

  .card-date {
    font-size: 0.775rem;
    color: var(--muted);
    margin: 0;
  }
</style>
