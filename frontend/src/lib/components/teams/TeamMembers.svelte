<script lang="ts">
  import type { TeamMember } from '$lib/types';

  let {
    members,
    canRemove = false,
    canMove = false,
    otherTeams = [],
    onRemove = (_userID: string) => {},
    onMove = (_userID: string, _toTeamID: string) => {}
  }: {
    members: TeamMember[];
    canRemove?: boolean;
    canMove?: boolean;
    otherTeams?: { id: string; name: string }[];
    onRemove?: (userID: string) => void;
    onMove?: (userID: string, toTeamID: string) => void;
  } = $props();

  const roleLabel: Record<string, string> = {
    profesor: 'Profesor',
    alumno: 'Alumno'
  };

  const roleBadgeClass: Record<string, string> = {
    profesor: 'badge-profesor',
    alumno: 'badge-alumno'
  };

  // Equipo destino elegido para cada miembro, indexado por user_id.
  let moveTarget: Record<string, string> = $state({});
</script>

<div class="members">
  {#if members.length === 0}
    <p class="empty-msg">Este equipo no tiene miembros todavía.</p>
  {:else}
    <table class="table">
      <thead>
        <tr>
          <th>Nombre</th>
          <th>Correo</th>
          <th>Rol</th>
          {#if canMove}<th>Mover a</th>{/if}
          {#if canRemove}<th></th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each members as member}
          <tr>
            <td class="td-name">{member.display_name}</td>
            <td class="td-email">{member.email}</td>
            <td>
              <span class="role-badge {roleBadgeClass[member.role] ?? 'badge-alumno'}">
                {roleLabel[member.role] ?? member.role}
              </span>
            </td>
            {#if canMove}
              <td>
                {#if otherTeams.length === 0}
                  <span class="no-teams">Sin otros equipos</span>
                {:else}
                  <div class="move-control">
                    <select bind:value={moveTarget[member.user_id]} class="move-select">
                      <option value="">Elegir equipo…</option>
                      {#each otherTeams as team}
                        <option value={team.id}>{team.name}</option>
                      {/each}
                    </select>
                    <button
                      class="btn-move"
                      disabled={!moveTarget[member.user_id]}
                      onclick={() => onMove(member.user_id, moveTarget[member.user_id])}
                    >
                      Mover
                    </button>
                  </div>
                {/if}
              </td>
            {/if}
            {#if canRemove}
              <td>
                <button
                  class="btn-remove"
                  onclick={() => onRemove(member.user_id)}
                  title="Remover miembro"
                >
                  ✕
                </button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .members { width: 100%; }

  .empty-msg {
    color: var(--muted);
    font-size: 0.875rem;
    padding: 1rem 0;
  }

  .table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
  }

  th {
    text-align: left;
    padding: 0.6rem 0.75rem;
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--muted);
    border-bottom: 1px solid var(--border);
  }

  td {
    padding: 0.75rem;
    border-bottom: 1px solid var(--border);
    color: var(--text);
  }

  tr:last-child td { border-bottom: none; }

  .td-name { font-weight: 500; }
  .td-email { color: var(--muted); }

  .role-badge {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 700;
    padding: 0.15rem 0.5rem;
    border-radius: 20px;
    text-transform: uppercase;
    letter-spacing: 0.4px;
  }

  .badge-profesor { background: #d1fae5; color: #065f46; }
  .badge-alumno   { background: color-mix(in srgb, var(--muted) 15%, transparent);  color: var(--muted); }

  .move-control {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .move-select {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.3rem 0.5rem;
    font-size: 0.8rem;
    color: var(--text);
  }

  .no-teams {
    font-size: 0.75rem;
    color: var(--muted);
    font-style: italic;
  }

  .btn-move {
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.3rem 0.65rem;
    font-size: 0.75rem;
    color: var(--purple);
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
  }

  .btn-move:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-move:not(:disabled):hover { border-color: var(--purple); }

  .btn-remove {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 0.875rem;
    padding: 0.25rem 0.5rem;
    border-radius: var(--radius-sm);
    transition: all 0.15s;
  }

  .btn-remove:hover {
    color: #dc2626;
    background: #fef2f2;
  }
</style>
